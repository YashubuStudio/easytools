package util

import "strings"

func JoinPathLike(a, b string) string {
	if !strings.HasPrefix(a, "/") {
		a = "/" + a
	}
	if !strings.HasPrefix(b, "/") {
		b = "/" + b
	}
	return strings.TrimRight(a, "/") + b
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
