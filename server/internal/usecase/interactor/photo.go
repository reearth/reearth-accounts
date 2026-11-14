package interactor

import (
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
)

// photoPath returns the storage path to save for a photo. Responses sign the
// stored value, so an absolute URL would come back as a broken signed link.
func photoPath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	lower := strings.ToLower(p)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return "", interfaces.ErrInvalidPhotoURL
	}
	return p, nil
}
