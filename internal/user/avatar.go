package user

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"net/http"

	"github.com/charmbracelet/log"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/storage"
)

const (
	MaxAvatarBytes  = 5 << 20
	maxAvatarPixels = 40_000_000
	avatarSide      = 512
)

var (
	errNotImage        = apperr.BadRequest("avatar must be a JPEG, PNG, WebP or GIF image")
	errTooManyPixels   = apperr.BadRequest("avatar dimensions are too large")
	errUploadsDisabled = apperr.Unavailable("image uploads are not configured")
	ErrAvatarTooLarge  = apperr.New(http.StatusRequestEntityTooLarge, fmt.Sprintf("avatar must be at most %d MB", MaxAvatarBytes>>20))
	errNoAvatar        = apperr.NotFound("no profile picture to remove")
)

// processAvatar decodes the upload, centre-crops it square and re-encodes it at most avatarSide pixels wide.
// Re-encoding drops EXIF data (such as GPS location) and anything that is not image data.
func processAvatar(raw []byte) (data []byte, contentType, ext string, err error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, "", "", errNotImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxAvatarPixels {
		return nil, "", "", errTooManyPixels
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", "", errNotImage
	}

	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(0, 0, side, side).Add(image.Pt(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2))
	out := min(side, avatarSide)
	dst := image.NewRGBA(image.Rect(0, 0, out, out))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, draw.Src, nil)

	var buf bytes.Buffer
	if format == "jpeg" {
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85})
		contentType, ext = "image/jpeg", "jpg"
	} else {
		err = png.Encode(&buf, dst)
		contentType, ext = "image/png", "png"
	}
	if err != nil {
		return nil, "", "", err
	}
	return buf.Bytes(), contentType, ext, nil
}

type Avatars struct {
	users *Store
	files *storage.Client
}

func NewAvatars(users *Store, files *storage.Client) *Avatars {
	return &Avatars{users: users, files: files}
}

// Upload stores the new picture, points the profile at it, then deletes the previous upload.
func (a *Avatars) Upload(ctx context.Context, userID bson.ObjectID, raw []byte) (*User, error) {
	if a.files == nil {
		return nil, errUploadsDisabled
	}
	data, contentType, ext, err := processAvatar(raw)
	if err != nil {
		return nil, err
	}
	before, err := a.users.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("avatars/%s/%s.%s", userID.Hex(), hex.EncodeToString(suffix), ext)
	url, err := a.files.Put(ctx, key, contentType, data)
	if err != nil {
		return nil, err
	}

	u, err := a.users.SetAvatar(ctx, userID, url, key)
	if err != nil {
		a.deleteQuietly(ctx, key)
		return nil, err
	}
	a.deleteQuietly(ctx, before.Profile.AvatarKey)
	return u, nil
}

func (a *Avatars) Remove(ctx context.Context, userID bson.ObjectID) (*User, error) {
	before, err := a.users.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if before.Profile.AvatarURL == "" {
		return nil, errNoAvatar
	}
	u, err := a.users.ClearAvatar(ctx, userID)
	if err != nil {
		return nil, err
	}
	a.deleteQuietly(ctx, before.Profile.AvatarKey)
	return u, nil
}

// An orphaned object only costs storage, so a failed delete is logged rather than failing the request.
func (a *Avatars) deleteQuietly(ctx context.Context, key string) {
	if key == "" || a.files == nil {
		return
	}
	if err := a.files.Delete(ctx, key); err != nil {
		log.Warn("could not delete old avatar", "key", key, "err", err)
	}
}
