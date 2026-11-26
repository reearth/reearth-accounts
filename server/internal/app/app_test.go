package app

import (
	"fmt"
	"net/http"
	"testing"

	adapterhttp "github.com/reearth/reearth-accounts/server/internal/adapter/http"
	"github.com/stretchr/testify/assert"
)

func TestErrorMessageMapsRESTRefusals(t *testing.T) {
	code, msg := errorMessage(fmt.Errorf("%w: closed", adapterhttp.ErrForbidden), func(error) {})
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "forbidden: closed", msg)

	code, _ = errorMessage(adapterhttp.ErrUnauthorized, func(error) {})
	assert.Equal(t, http.StatusUnauthorized, code)
}
