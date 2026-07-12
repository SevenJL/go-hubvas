package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

type memoryMediaRepo struct {
	uploads map[string]Upload
	avatar  *AvatarResponse
	oldKey  string
}

func newMemoryMediaRepo() *memoryMediaRepo { return &memoryMediaRepo{uploads: map[string]Upload{}} }
func (r *memoryMediaRepo) CreateUpload(_ context.Context, upload Upload) error {
	r.uploads[upload.ID] = upload
	return nil
}
func (r *memoryMediaRepo) GetUpload(_ context.Context, id string, user identity.UserID) (Upload, error) {
	upload, ok := r.uploads[id]
	if !ok || upload.UserID != user {
		return Upload{}, shared.NewDomainError(shared.ErrNotFound, "upload not found")
	}
	return upload, nil
}
func (r *memoryMediaRepo) FinalizeAvatar(_ context.Context, id string, user identity.UserID, _ string, url string, version int64) (string, error) {
	upload, ok := r.uploads[id]
	if !ok || upload.UserID != user {
		return "", shared.NewDomainError(shared.ErrNotFound, "upload not found")
	}
	if upload.State != "pending" {
		return "", shared.NewDomainError(shared.ErrConflict, "upload is not pending")
	}
	upload.State = "completed"
	r.uploads[id] = upload
	r.avatar = &AvatarResponse{AvatarURL: url, AvatarVersion: version}
	return r.oldKey, nil
}
func (r *memoryMediaRepo) RemoveAvatar(context.Context, identity.UserID) (string, error) {
	old := r.oldKey
	r.oldKey = ""
	r.avatar = &AvatarResponse{}
	return old, nil
}
func (r *memoryMediaRepo) CurrentAvatar(context.Context, identity.UserID) (*AvatarResponse, error) {
	if r.avatar == nil {
		return &AvatarResponse{}, nil
	}
	copy := *r.avatar
	return &copy, nil
}
func (r *memoryMediaRepo) ExpiredUploads(_ context.Context, limit int) ([]Upload, error) {
	var uploads []Upload
	for _, upload := range r.uploads {
		if len(uploads) == limit {
			break
		}
		if (upload.State == "pending" || upload.State == "failed") && upload.ExpiresAt.Before(time.Now()) {
			uploads = append(uploads, upload)
		}
	}
	return uploads, nil
}
func (r *memoryMediaRepo) DeleteUpload(_ context.Context, id string) error {
	delete(r.uploads, id)
	return nil
}

type memoryMediaStore struct {
	objects map[string][]byte
	deleted []string
}

func newMemoryMediaStore() *memoryMediaStore { return &memoryMediaStore{objects: map[string][]byte{}} }
func (s *memoryMediaStore) PresignPut(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	return "https://upload.invalid/" + key, nil
}
func (s *memoryMediaStore) Put(_ context.Context, key string, reader io.Reader, size int64, _ string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if int64(len(data)) != size {
		return errors.New("size mismatch")
	}
	s.objects[key] = data
	return nil
}
func (s *memoryMediaStore) Get(_ context.Context, key string) (io.ReadCloser, int64, string, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, 0, "", errors.New("object missing")
	}
	return io.NopCloser(bytes.NewReader(data)), int64(len(data)), "application/octet-stream", nil
}
func (s *memoryMediaStore) Delete(_ context.Context, key string) error {
	delete(s.objects, key)
	s.deleted = append(s.deleted, key)
	return nil
}
func (s *memoryMediaStore) PublicURL(key string) string { return "/media/" + key }

func pngBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 150, A: 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func pngWithDeclaredDimensions(t *testing.T, width, height uint32) []byte {
	t.Helper()
	data := append([]byte(nil), pngBytes(t, 1, 1)...)
	binary.BigEndian.PutUint32(data[16:20], width)
	binary.BigEndian.PutUint32(data[20:24], height)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	return data
}

func jpegWithOrientation(t *testing.T, width, height, orientation int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 10), G: uint8(y * 10), B: 120, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	// Minimal big-endian TIFF payload containing one Orientation SHORT entry.
	tiff := make([]byte, 26)
	copy(tiff[0:2], "MM")
	binary.BigEndian.PutUint16(tiff[2:4], 42)
	binary.BigEndian.PutUint32(tiff[4:8], 8)
	binary.BigEndian.PutUint16(tiff[8:10], 1)
	binary.BigEndian.PutUint16(tiff[10:12], 0x0112)
	binary.BigEndian.PutUint16(tiff[12:14], 3)
	binary.BigEndian.PutUint32(tiff[14:18], 1)
	binary.BigEndian.PutUint16(tiff[18:20], uint16(orientation))
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xff, 0xe1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	segment = append(segment, payload...)
	jpegData := encoded.Bytes()
	result := append([]byte(nil), jpegData[:2]...)
	result = append(result, segment...)
	return append(result, jpegData[2:]...)
}

func prepareUpload(repo *memoryMediaRepo, store *memoryMediaStore, user identity.UserID, data []byte, state string, expiry time.Time) Upload {
	upload := Upload{ID: "upload-1", UserID: user, TempKey: "tmp/avatars/1/upload-1", ContentType: "image/png", ExpectedSize: int64(len(data)), State: state, ExpiresAt: expiry}
	repo.uploads[upload.ID] = upload
	store.objects[upload.TempKey] = data
	return upload
}

func TestCompleteRejectsSpoofedImageAndInvalidCrop(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryMediaRepo()
	store := newMemoryMediaStore()
	svc := NewService(repo, store, time.Minute, 5<<20)
	prepareUpload(repo, store, 1, []byte("not an image"), "pending", time.Now().Add(time.Minute))
	if _, err := svc.Complete(ctx, 1, CompleteRequest{UploadID: "upload-1"}); !errors.Is(err, shared.ErrInvalidArgument) {
		t.Fatalf("expected invalid image error, got %v", err)
	}

	data := pngBytes(t, 10, 8)
	prepareUpload(repo, store, 1, data, "pending", time.Now().Add(time.Minute))
	_, err := svc.Complete(ctx, 1, CompleteRequest{UploadID: "upload-1", Crop: &Crop{X: .8, Y: 0, Width: .4, Height: 1}})
	if !errors.Is(err, shared.ErrInvalidArgument) {
		t.Fatalf("expected invalid crop error, got %v", err)
	}

	prepareUpload(repo, store, 1, data, "pending", time.Now().Add(time.Minute))
	_, err = svc.Complete(ctx, 1, CompleteRequest{UploadID: "upload-1", Crop: &Crop{X: 0, Y: 0, Width: .4, Height: .75}})
	if !errors.Is(err, shared.ErrInvalidArgument) {
		t.Fatalf("expected non-square crop error, got %v", err)
	}
}

func TestCompleteAppliesJPEGEXIFOrientationBeforeCrop(t *testing.T) {
	repo := newMemoryMediaRepo()
	store := newMemoryMediaStore()
	svc := NewService(repo, store, time.Minute, 5<<20)
	// Stored pixels are 12x8, but Orientation=6 makes the browser preview 8x12.
	// The browser therefore sends a full-width crop whose normalized height is
	// 8/12. Applying that crop to the unrotated matrix would be non-square.
	data := jpegWithOrientation(t, 12, 8, 6)
	prepareUpload(repo, store, 1, data, "pending", time.Now().Add(time.Minute))
	result, err := svc.Complete(context.Background(), 1, CompleteRequest{
		UploadID: "upload-1",
		Crop:     &Crop{X: 0, Y: 0.2, Width: 1, Height: 8.0 / 12.0},
	})
	if err != nil {
		t.Fatalf("EXIF-oriented browser crop should succeed: %v", err)
	}
	if !strings.HasSuffix(result.AvatarURL, ".webp") {
		t.Fatalf("expected processed WebP avatar, got %q", result.AvatarURL)
	}
}

func TestCompleteRejectsDeclaredPixelBombBeforeFullDecode(t *testing.T) {
	repo := newMemoryMediaRepo()
	store := newMemoryMediaStore()
	svc := NewService(repo, store, time.Minute, 5<<20)
	data := pngWithDeclaredDimensions(t, 50_000, 1_000)
	prepareUpload(repo, store, 1, data, "pending", time.Now().Add(time.Minute))
	if _, err := svc.Complete(context.Background(), 1, CompleteRequest{UploadID: "upload-1"}); !errors.Is(err, shared.ErrLimitExceeded) {
		t.Fatalf("expected pixel-limit error, got %v", err)
	}
}

func TestCompleteIsUserScopedAndIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryMediaRepo()
	store := newMemoryMediaStore()
	svc := NewService(repo, store, time.Minute, 5<<20)
	data := pngBytes(t, 16, 12)
	prepareUpload(repo, store, 7, data, "pending", time.Now().Add(time.Minute))
	if _, err := svc.Complete(ctx, 8, CompleteRequest{UploadID: "upload-1"}); !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("cross-user completion should be hidden, got %v", err)
	}
	first, err := svc.Complete(ctx, 7, CompleteRequest{UploadID: "upload-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(first.AvatarURL, ".webp") {
		t.Fatalf("expected WebP avatar URL, got %q", first.AvatarURL)
	}
	second, err := svc.Complete(ctx, 7, CompleteRequest{UploadID: "upload-1"})
	if err != nil {
		t.Fatal(err)
	}
	if second.AvatarURL != first.AvatarURL || second.AvatarVersion != first.AvatarVersion {
		t.Fatalf("completed request was not idempotent: %#v %#v", first, second)
	}
}

func TestCompleteRejectsExpiredAndMismatchedSize(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryMediaRepo()
	store := newMemoryMediaStore()
	svc := NewService(repo, store, time.Minute, 5<<20)
	data := pngBytes(t, 4, 4)
	prepareUpload(repo, store, 1, data, "pending", time.Now().Add(-time.Second))
	if _, err := svc.Complete(ctx, 1, CompleteRequest{UploadID: "upload-1"}); !errors.Is(err, shared.ErrConflict) {
		t.Fatalf("expected expired conflict, got %v", err)
	}
	upload := prepareUpload(repo, store, 1, data, "pending", time.Now().Add(time.Minute))
	upload.ExpectedSize++
	repo.uploads[upload.ID] = upload
	if _, err := svc.Complete(ctx, 1, CompleteRequest{UploadID: "upload-1"}); !errors.Is(err, shared.ErrInvalidArgument) {
		t.Fatalf("expected size mismatch, got %v", err)
	}
}

func TestMultipartAndCleanupUseSharedProcessing(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryMediaRepo()
	store := newMemoryMediaStore()
	svc := NewService(repo, store, time.Minute, 5<<20)
	data := pngBytes(t, 12, 18)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "avatar.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(data); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err = req.ParseMultipartForm(6 << 20); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Multipart(ctx, 3, req.MultipartForm.File["file"][0], nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(result.AvatarURL, ".webp") {
		t.Fatalf("multipart did not use WebP processing: %q", result.AvatarURL)
	}

	expired := Upload{ID: "expired", UserID: 3, TempKey: "tmp/avatars/3/expired", State: "pending", ExpiresAt: time.Now().Add(-time.Hour)}
	repo.uploads[expired.ID] = expired
	store.objects[expired.TempKey] = []byte("old")
	if err := svc.CleanupExpired(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.uploads[expired.ID]; ok {
		t.Fatal("expired upload record was not removed")
	}
	if _, ok := store.objects[expired.TempKey]; ok {
		t.Fatal("expired temporary object was not removed")
	}
}
