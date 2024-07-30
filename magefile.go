//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	packageName          = "github.com/alex0x08/ungoogled-go"
	buildWindowsGUIflags = "-H windowsgui -X main.DebugMode=false"
)

var ldflags = buildWindowsGUIflags

// allow user to override go executable by running as GOEXE=xxx make ... on unix-like systems
var goexe = "go"

func init() {
	if exe := os.Getenv("GOEXE"); exe != "" {
		goexe = exe
	}
}

func Generate() error {
	return runWith(flagEnv(), goexe, "generate")
}
func Install() error {
	return runWith(flagEnv(), goexe, "install")
}

// Build our binary
func Build() error {
	return runWith(flagEnv(), goexe, "build", "-ldflags",
		ldflags, buildFlags(), "-tags", buildTags(), packageName)
}

// Uninstall hugo binary
func Clean() error {
	return sh.Run(goexe, "clean", "-i", packageName)
}

func flagEnv() map[string]string {
	hash, _ := sh.Output("git", "rev-parse", "--short", "HEAD")
	return map[string]string{
		"PACKAGE":     packageName,
		"COMMIT_HASH": hash,
		"BUILD_DATE":  time.Now().Format("2006-01-02T15:04:05Z0700"),
	}
}

func buildFlags() []string {
	if runtime.GOOS == "windows" {
		return []string{"-buildmode", "exe"}
	}
	return nil
}

func buildTags() string {
	// To build the extended Hugo SCSS/SASS enabled version, build with
	// HUGO_BUILD_TAGS=extended mage install etc.
	// To build without `hugo deploy` for smaller binary, use HUGO_BUILD_TAGS=nodeploy
	if envtags := os.Getenv("HUGO_BUILD_TAGS"); envtags != "" {
		return envtags
	}
	return "none"
}

func runWith(env map[string]string, cmd string, inArgs ...any) error {
	s := argsToStrings(inArgs...)
	return sh.RunWith(env, cmd, s...)
}

func runCmd(env map[string]string, cmd string, args ...any) error {
	if mg.Verbose() {
		return runWith(env, cmd, args...)
	}
	output, err := sh.OutputWith(env, cmd, argsToStrings(args...)...)
	if err != nil {
		fmt.Fprint(os.Stderr, output)
	}

	return err
}

func argsToStrings(v ...any) []string {
	var args []string
	for _, arg := range v {
		switch v := arg.(type) {
		case string:
			if v != "" {
				args = append(args, v)
			}
		case []string:
			if v != nil {
				args = append(args, v...)
			}
		default:
			panic("invalid type")
		}
	}

	return args
}
