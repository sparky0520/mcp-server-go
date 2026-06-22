package tools

import (
	"fmt"
	"strings"
)

type HelloArgs struct {
	Name string `json:"name"`
}

type HelloResult struct {
	Message string `json:"message"`
}

func Hello(args HelloArgs) (HelloResult, error) {
	name := strings.TrimSpace(args.Name)
	if name == "" {
		name = "world"
	}

	return HelloResult{
		Message: fmt.Sprintf("Hello, %s", name),
	}, nil
}
