package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/versioneer-tech/package-r/files"
	"github.com/versioneer-tech/package-r/users"
)

type profileUpdateRequest struct {
	Locale       *string         `json:"locale"`
	ViewMode     *users.ViewMode `json:"viewMode"`
	SingleClick  *bool           `json:"singleClick"`
	Sorting      *files.Sorting  `json:"sorting"`
	HideDotfiles *bool           `json:"hideDotfiles"`
	DateFormat   *bool           `json:"dateFormat"`
}

var profilePatchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()

	var request profileUpdateRequest
	if err := decoder.Decode(&request); err != nil {
		return http.StatusBadRequest, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return http.StatusBadRequest, err
	}

	fields := make([]string, 0, 6)
	if request.Locale != nil {
		locale := strings.TrimSpace(*request.Locale)
		if locale == "" || len(locale) > 32 {
			return http.StatusBadRequest, nil
		}
		d.user.Locale = locale
		fields = append(fields, "Locale")
	}
	if request.ViewMode != nil {
		if *request.ViewMode != users.ListViewMode && *request.ViewMode != users.MosaicViewMode && *request.ViewMode != "mosaic gallery" {
			return http.StatusBadRequest, nil
		}
		d.user.ViewMode = *request.ViewMode
		fields = append(fields, "ViewMode")
	}
	if request.SingleClick != nil {
		d.user.SingleClick = *request.SingleClick
		fields = append(fields, "SingleClick")
	}
	if request.Sorting != nil {
		if request.Sorting.By != "name" && request.Sorting.By != "size" && request.Sorting.By != "modified" {
			return http.StatusBadRequest, nil
		}
		d.user.Sorting = *request.Sorting
		fields = append(fields, "Sorting")
	}
	if request.HideDotfiles != nil {
		d.user.HideDotfiles = *request.HideDotfiles
		fields = append(fields, "HideDotfiles")
	}
	if request.DateFormat != nil {
		d.user.DateFormat = *request.DateFormat
		fields = append(fields, "DateFormat")
	}
	if len(fields) == 0 {
		return http.StatusBadRequest, nil
	}

	if err := d.store.Users.Update(d.user, fields...); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusNoContent, nil
})
