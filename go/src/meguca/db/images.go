package db

import (
	"database/sql"
	"errors"
	"meguca/auth"
	"meguca/common"
	"meguca/imager/assets"
	"meguca/util"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

const (
	// Time it takes for an image allocation token to expire
	tokenTimeout = time.Minute
)

var (
	// ErrInvalidToken occurs, when trying to retrieve an image with an
	// non-existent token. The token might have expired (60 to 119 seconds) or
	// the client could have provided an invalid token to begin with.
	ErrInvalidToken = errors.New("invalid image token")
)

// WriteImage writes a processed image record to the DB
func WriteImage(i common.ImageCommon) error {
	dims := pq.GenericArray{A: i.Dims}
	_, err := sq.Insert("images").
		Columns(
			"apng", "audio", "video", "fileType", "thumbType", "dims", "length",
			"size", "MD5", "SHA1", "Title", "Artist",
		).
		Values(
			i.APNG, i.Audio, i.Video, int(i.FileType), int(i.ThumbType), dims,
			i.Length, i.Size, i.MD5, i.SHA1, i.Title, i.Artist,
		).
		Exec()
	return err
}

func getImage(sha1 string) squirrel.SelectBuilder {
	return sq.Select("*").From("images").Where("SHA1 = ?", sha1)
}

// GetImage retrieves a thumbnailed image record from the DB
func GetImage(SHA1 string) (common.ImageCommon, error) {
	return scanImage(getImage(SHA1))
}

func scanImage(rs rowScanner) (img common.ImageCommon, err error) {
	var scanner imageScanner
	err = rs.Scan(scanner.ScanArgs()...)
	if err != nil {
		return
	}
	return scanner.Val().ImageCommon, nil
}

// NewImageToken inserts a new image allocation token into the DB and returns
// it's ID
func NewImageToken(SHA1 string) (token string, err error) {
	expires := time.Now().Add(tokenTimeout)

	// Loop in case there is a primary key collision
	for {
		token, err = auth.RandomID(64)
		if err != nil {
			return
		}

		_, err = sq.Insert("image_tokens").
			Columns("token", "SHA1", "expires").
			Values(token, SHA1, expires).
			Exec()
		switch {
		case err == nil:
			return
		case IsConflictError(err):
			continue
		default:
			return
		}
	}
}

// UseImageToken deletes an image allocation token and returns the matching
// processed image. If no token exists, returns ErrInvalidToken.
func UseImageToken(tx *sql.Tx, token string) (
	img common.ImageCommon, err error,
) {
	if len(token) != common.LenImageToken {
		err = ErrInvalidToken
		return
	}

	var SHA1 string
	q := sq.Delete("image_tokens").
		Where("token = ?", token).
		Suffix("returning SHA1")
	r, err := withTransaction(tx, q).QueryRow()
	if err != nil {
		return
	}
	err = r.Scan(&SHA1)
	if err != nil {
		return
	}

	img, err = scanImage(getImage(SHA1))
	return
}

// AllocateImage allocates an image's file resources to their respective served
// directories and write its data to the database
func AllocateImage(src, thumb []byte, img common.ImageCommon) error {
	err := assets.Write(img.SHA1, img.FileType, img.ThumbType, src, thumb)
	if err != nil {
		return cleanUpFailedAllocation(img, err)
	}

	err = WriteImage(img)
	if err != nil {
		return cleanUpFailedAllocation(img, err)
	}
	return nil
}

// Delete any dangling image files in case of a failed image allocation
func cleanUpFailedAllocation(img common.ImageCommon, err error) error {
	delErr := assets.Delete(img.SHA1, img.FileType, img.ThumbType)
	if delErr != nil {
		err = util.WrapError(err.Error(), delErr)
	}
	return err
}

// HasImage returns, if the post has an image allocated. Only used in tests.
func HasImage(id uint64) (has bool, err error) {
	err = sq.Select("true").
		From("posts").
		Where("id = ? and SHA1 IS NOT NULL", id).
		QueryRow().
		Scan(&has)
	if err == sql.ErrNoRows {
		err = nil
	}
	return
}

// InsertImage insert and image into and existing open post
func InsertImage(tx *sql.Tx, id, op uint64, img common.Image) (err error) {
	q := sq.Update("posts").
		SetMap(map[string]interface{}{
			"SHA1":      img.SHA1,
			"imageName": img.Name,
			"spoiler":   img.Spoiler,
		}).
		Where("id = ?", id)
	err = withTransaction(tx, q).Exec()
	if err != nil {
		return
	}
	return bumpThread(tx, op, false)
}

// SpoilerImage spoilers an already allocated image
func SpoilerImage(id, op uint64) error {
	return InTransaction(func(tx *sql.Tx) (err error) {
		err = withTransaction(tx,
			sq.Update("posts").
				Set("spoiler", true).
				Where("id = ?", id),
		).
			Exec()
		if err != nil {
			return
		}

		return bumpThread(tx, op, false)
	})
}

// Delete an image as part of clearing a post
func DeleteOwnedImage(id uint64) error {
	_, err := sq.Update("posts").
		Set("SHA1", nil).
		Where("id = ?", id).
		Exec()
	return err
}

// Returns random video ID by board
func RandomVideo(board string) (sha1 string, err error) {
	err = sq.Select("p.SHA1").
		From("posts as p").
		Join("images as i on i.SHA1 = p.SHA1").
		Where(squirrel.Eq{
			"p.board":    board,
			"i.audio":    true,
			"i.fileType": int(common.WEBM),
		}).
		OrderBy("RANDOM()").
		Limit(1).
		QueryRow().
		Scan(&sha1)
	return
}
