package midplat

import (
	"errors"
	"testing"
)

func TestIsDataNotExist(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"http DATA_NOT_EXIST", &HTTPError{Status: 400, Code: "DATA_NOT_EXIST"}, true},
		{"api DATA_NOT_EXIST", &APIError{Code: "DATA_NOT_EXIST"}, true},
		{"http other code", &HTTPError{Status: 400, Code: "INVALID_PARAM"}, false},
		{"http no code", &HTTPError{Status: 500}, false},
		{"plain error", errors.New("boom"), false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		if got := IsDataNotExist(c.err); got != c.want {
			t.Errorf("%s: IsDataNotExist=%v want %v", c.name, got, c.want)
		}
	}
}
