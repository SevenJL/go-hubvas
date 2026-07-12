package media

import (
	"bytes"
	"context"
	"fmt"
	"github.com/HugoSmits86/nativewebp"
	"github.com/google/uuid"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"strings"
	"time"
)

var _ = jpeg.DefaultQuality
var _ = png.BestCompression

type Crop struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
type Upload struct {
	ID                          string
	UserID                      identity.UserID
	TempKey, ContentType, State string
	ExpectedSize                int64
	ExpiresAt                   time.Time
}
type PresignRequest struct {
	ContentType string `json:"content_type" binding:"required"`
	Size        int64  `json:"size" binding:"required,min=1"`
}
type CompleteRequest struct {
	UploadID string `json:"upload_id" binding:"required"`
	Crop     *Crop  `json:"crop,omitempty"`
}
type PresignResponse struct {
	UploadID  string            `json:"upload_id"`
	UploadURL string            `json:"upload_url"`
	ExpiresAt time.Time         `json:"expires_at"`
	Headers   map[string]string `json:"headers"`
}
type AvatarResponse struct {
	AvatarURL     string `json:"avatar_url"`
	AvatarVersion int64  `json:"avatar_version"`
}
type Repository interface {
	CreateUpload(context.Context, Upload) error
	GetUpload(context.Context, string, identity.UserID) (Upload, error)
	FinalizeAvatar(context.Context, string, identity.UserID, string, string, int64) (string, error)
	RemoveAvatar(context.Context, identity.UserID) (string, error)
	CurrentAvatar(context.Context, identity.UserID) (*AvatarResponse, error)
	ExpiredUploads(context.Context, int) ([]Upload, error)
	DeleteUpload(context.Context, string) error
}
type Store interface {
	PresignPut(context.Context, string, string, time.Duration) (string, error)
	Put(context.Context, string, io.Reader, int64, string) error
	Get(context.Context, string) (io.ReadCloser, int64, string, error)
	Delete(context.Context, string) error
	PublicURL(string) string
}
type Service struct {
	repo     Repository
	store    Store
	ttl      time.Duration
	maxBytes int64
}

func NewService(r Repository, s Store, ttl time.Duration, max int64) *Service {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if max <= 0 {
		max = 5 << 20
	}
	return &Service{repo: r, store: s, ttl: ttl, maxBytes: max}
}

func (s *Service) MaxUploadBytes() int64 { return s.maxBytes }

func allowedType(v string) bool { return v == "image/jpeg" || v == "image/png" || v == "image/webp" }
func (s *Service) Presign(ctx context.Context, user identity.UserID, in PresignRequest) (*PresignResponse, error) {
	if !allowedType(in.ContentType) {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "only JPEG, PNG and WebP avatars are allowed")
	}
	if in.Size <= 0 || in.Size > s.maxBytes {
		return nil, shared.NewDomainError(shared.ErrLimitExceeded, "avatar must be no larger than 5 MB")
	}
	id := uuid.NewString()
	key := fmt.Sprintf("tmp/avatars/%d/%s", user, id)
	expires := time.Now().Add(s.ttl)
	if err := s.repo.CreateUpload(ctx, Upload{ID: id, UserID: user, TempKey: key, ContentType: in.ContentType, ExpectedSize: in.Size, State: "pending", ExpiresAt: expires}); err != nil {
		return nil, err
	}
	u, err := s.store.PresignPut(ctx, key, in.ContentType, s.ttl)
	if err != nil {
		return nil, err
	}
	return &PresignResponse{UploadID: id, UploadURL: u, ExpiresAt: expires, Headers: map[string]string{"Content-Type": in.ContentType}}, nil
}
func (s *Service) Multipart(ctx context.Context, user identity.UserID, file *multipart.FileHeader, crop *Crop) (*AvatarResponse, error) {
	if file.Size <= 0 || file.Size > s.maxBytes {
		return nil, shared.NewDomainError(shared.ErrLimitExceeded, "avatar must be no larger than 5 MB")
	}
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	id := uuid.NewString()
	key := fmt.Sprintf("tmp/avatars/%d/%s", user, id)
	ct := file.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	if err = s.repo.CreateUpload(ctx, Upload{ID: id, UserID: user, TempKey: key, ContentType: ct, ExpectedSize: file.Size, State: "pending", ExpiresAt: time.Now().Add(s.ttl)}); err != nil {
		return nil, err
	}
	if err = s.store.Put(ctx, key, f, file.Size, ct); err != nil {
		return nil, err
	}
	return s.Complete(ctx, user, CompleteRequest{UploadID: id, Crop: crop})
}
func (s *Service) Complete(ctx context.Context, user identity.UserID, in CompleteRequest) (*AvatarResponse, error) {
	up, err := s.repo.GetUpload(ctx, in.UploadID, user)
	if err != nil {
		return nil, err
	}
	if up.State == "completed" {
		return s.repo.CurrentAvatar(ctx, user)
	}
	if time.Now().After(up.ExpiresAt) {
		return nil, shared.NewDomainError(shared.ErrConflict, "upload has expired")
	}
	rc, size, _, err := s.store.Get(ctx, up.TempKey)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	if size <= 0 || size > s.maxBytes || size != up.ExpectedSize {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "uploaded object size does not match request")
	}
	limited := io.LimitReader(rc, s.maxBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "avatar is not a valid image")
	}
	if format != "jpeg" && format != "png" && format != "webp" {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "unsupported avatar image format")
	}
	pixels := int64(config.Width) * int64(config.Height)
	if config.Width < 1 || config.Height < 1 || pixels > 40_000_000 {
		return nil, shared.NewDomainError(shared.ErrLimitExceeded, "avatar dimensions are too large")
	}
	// Decode only after checking dimensions so a small compressed payload cannot
	// force an unbounded pixel allocation.
	img, decodedFormat, err := image.Decode(bytes.NewReader(raw))
	if err != nil || decodedFormat != format {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "avatar is not a valid image")
	}
	b := img.Bounds()
	if in.Crop != nil && !validCrop(in.Crop) {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "invalid avatar crop")
	}
	src := cropRect(b, in.Crop)
	if in.Crop != nil && absInt(src.Dx()-src.Dy()) > 1 {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "avatar crop must be square")
	}
	dst := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, src, draw.Over, nil)
	var out bytes.Buffer
	if err = nativewebp.Encode(&out, dst, &nativewebp.Options{UseExtendedFormat: true}); err != nil {
		return nil, fmt.Errorf("encode avatar: %w", err)
	}
	version := time.Now().UnixNano()
	key := fmt.Sprintf("avatars/%d/%d.webp", user, version)
	if err = s.store.Put(ctx, key, bytes.NewReader(out.Bytes()), int64(out.Len()), "image/webp"); err != nil {
		return nil, err
	}
	url := s.store.PublicURL(key)
	old, err := s.repo.FinalizeAvatar(ctx, in.UploadID, user, key, url, version)
	if err != nil {
		_ = s.store.Delete(ctx, key)
		return nil, err
	}
	_ = s.store.Delete(ctx, up.TempKey)
	if old != "" && old != key {
		_ = s.store.Delete(context.Background(), old)
	}
	return &AvatarResponse{AvatarURL: url, AvatarVersion: version}, nil
}
func cropRect(b image.Rectangle, c *Crop) image.Rectangle {
	w, h := b.Dx(), b.Dy()
	if c != nil && validCrop(c) {
		x := b.Min.X + int(c.X*float64(w))
		y := b.Min.Y + int(c.Y*float64(h))
		cw := int(c.Width * float64(w))
		ch := int(c.Height * float64(h))
		r := image.Rect(x, y, x+cw, y+ch).Intersect(b)
		if r.Dx() > 0 && r.Dy() > 0 {
			return r
		}
	}
	side := w
	if h < side {
		side = h
	}
	return image.Rect(b.Min.X+(w-side)/2, b.Min.Y+(h-side)/2, b.Min.X+(w-side)/2+side, b.Min.Y+(h-side)/2+side)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
func validCrop(c *Crop) bool {
	return c.X >= 0 && c.Y >= 0 && c.Width > 0 && c.Height > 0 && c.X+c.Width <= 1.000001 && c.Y+c.Height <= 1.000001
}
func (s *Service) Remove(ctx context.Context, user identity.UserID) error {
	old, err := s.repo.RemoveAvatar(ctx, user)
	if err != nil {
		return err
	}
	if strings.TrimSpace(old) != "" {
		_ = s.store.Delete(context.Background(), old)
	}
	return nil
}

// CleanupExpired removes abandoned private temporary objects and their records.
func (s *Service) CleanupExpired(ctx context.Context, limit int) error {
	uploads, err := s.repo.ExpiredUploads(ctx, limit)
	if err != nil {
		return err
	}
	for _, upload := range uploads {
		if err := s.store.Delete(ctx, upload.TempKey); err != nil {
			continue
		}
		_ = s.repo.DeleteUpload(ctx, upload.ID)
	}
	return nil
}
