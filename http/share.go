package http

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	fbErrors "github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/fileutils"
	"github.com/versioneer-tech/package-r/share"
)

func withPermShare(fn handleFunc) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Share {
			return http.StatusForbidden, nil
		}

		return fn(w, r, d)
	})
}

var shareListHandler = withPermShare(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var (
		s   []*share.Link
		err error
	)
	if d.user.Perm.Admin {
		s, err = d.store.Share.All()
	} else {
		s, err = d.store.Share.FindByUserID(d.user.ID)
	}
	if errors.Is(err, fbErrors.ErrNotExist) {
		return renderJSON(w, r, []*share.Link{})
	}

	if err != nil {
		return http.StatusInternalServerError, err
	}

	sort.Slice(s, func(i, j int) bool {
		if s[i].UserID != s[j].UserID {
			return s[i].UserID < s[j].UserID
		}
		return s[i].Expire < s[j].Expire
	})

	return renderJSON(w, r, s)
})

var shareGetsHandler = withPermShare(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	s, err := d.store.Share.Gets(r.URL.Path, d.user.ID)
	if errors.Is(err, fbErrors.ErrNotExist) {
		return renderJSON(w, r, []*share.Link{})
	}

	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, s)
})

var shareDeleteHandler = withPermShare(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
	hash := strings.TrimSuffix(r.URL.Path, "/")
	hash = strings.TrimPrefix(hash, "/")

	if hash == "" {
		return http.StatusBadRequest, nil
	}

	err := d.store.Share.Delete(hash)
	return errToStatus(err), err
})

var sharePostHandler = withPermShare(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var s *share.Link
	var body share.CreateBody
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
		}
		defer r.Body.Close()
	}

	bytes := make([]byte, 6) //nolint:gomnd
	_, err := rand.Read(bytes)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	hash := base64.URLEncoding.EncodeToString(bytes)

	if body.Grant == "" {
		hash = "public-" + hash // optimization so we can pass x-id-token only on necessary routes
	}

	var expire int64 = 0

	if body.Expires != "" {
		//nolint:govet
		num, err := strconv.Atoi(body.Expires)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		var add time.Duration
		switch body.Unit {
		case "seconds":
			add = time.Second * time.Duration(num)
		case "minutes":
			add = time.Minute * time.Duration(num)
		case "days":
			add = time.Hour * 24 * time.Duration(num)
		default:
			add = time.Hour * time.Duration(num)
		}

		expire = time.Now().Add(add).Unix()
	}

	passwordHash, status, err := getSharePasswordHash(body)
	if err != nil {
		return status, err
	}

	var token string
	if len(passwordHash) > 0 {
		tokenBuffer := make([]byte, 96) //nolint:gomnd
		if _, err := rand.Read(tokenBuffer); err != nil {
			return http.StatusInternalServerError, err
		}
		token = base64.URLEncoding.EncodeToString(tokenBuffer)
	}

	path := r.URL.Path
	if body.Mode == "indexed" { //nolint:goconst,nolintlint
		path = "/packages/." + hash
		if err := fileutils.Copy(d.user.Fs, r.URL.Path, path, true); err != nil {
			return http.StatusInternalServerError, err
		}
	}

	s = &share.Link{
		Path:         path,
		Hash:         hash,
		Expire:       expire,
		Description:  body.Description,
		Creation:     time.Now().Unix(),
		UserID:       d.user.ID,
		PasswordHash: string(passwordHash),
		Token:        token,
		Grant:        body.Grant,
		Mode:         body.Mode,
	}

	if err := d.store.Share.Save(s); err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, s)
})

//nolint:gocritic
func getSharePasswordHash(body share.CreateBody) (data []byte, statuscode int, err error) {
	if body.Password == "" {
		return nil, 0, nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to hash password: %w", err)
	}

	return hash, 0, nil
}
