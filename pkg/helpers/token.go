package helpers

import (
	"encoding/base64"
	"net/http"
	"strings"
)

func GetTokenFromRequest(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	authList := strings.SplitN(authHeader, " ", 2)
	if len(authList) != 2 {
		return ""
	}

	switch strings.ToLower(authList[0]) {
	case "basic":
		upRaw, _ := base64.StdEncoding.DecodeString(authList[1])
		upList := strings.SplitN(string(upRaw), ":", 2)
		if len(upList) != 2 {
			return ""
		}
		return string(upList[1])

	case "bearer":
		return authList[1]

	default:
		return ""
	}
}
