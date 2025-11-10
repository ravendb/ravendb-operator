/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package validator

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type imageValidator struct {
	client client.Reader
}

func NewImageValidator(c client.Reader) *imageValidator {
	return &imageValidator{client: c}
}

func (v *imageValidator) Name() string {
	return "image-validator"
}

func (v *imageValidator) ValidateCreate(_ context.Context, c ClusterAdapter) error {
	image := c.GetImage()

	return validateImage(image)
}

func (v *imageValidator) ValidateUpdate(ctx context.Context, oldC, newC ClusterAdapter) error {
	// reuse all ValidateCreate validations for the new set image
	if err := v.ValidateCreate(ctx, newC); err != nil {
		return err
	}

	oldTag, _ := extractTag(oldC.GetImage())
	newTag, _ := extractTag(newC.GetImage())
	return compareTagsDowngrade(oldTag, newTag)
}

func init() {
	Register(&imageValidator{})
}

func validateImage(image string) error {
	if !isRavenRepo(image) {
		return fmt.Errorf("image must be under the 'ravendb/' registry namespace (e.g., ravendb/ravendb:<version>)")
	}
	if hasDigest(image) {
		return fmt.Errorf("digest references are not allowed. use a concrete tag (e.g., ':7.1.3-ubuntu.22.04-x64')")
	}
	tag, ok := extractTag(image)
	if !ok || tag == "" {
		return fmt.Errorf("image must specify a tag; implicit ':latest' is not allowed")
	}
	if isFloatingTag(tag) {
		return fmt.Errorf("floating tag %q is not allowed. use a concrete, pinned tag (e.g., ':7.1.3-ubuntu.22.04-x64')", tag)
	}
	if !isUbuntuTag(tag) {
		return fmt.Errorf("non-ubuntu images are not supported. use an ubuntu-tagged image (e.g., ':7.1.3-ubuntu.22.04-x64')")
	}
	return nil
}

func compareTagsDowngrade(oldTag, newTag string) error {
	oldVer, errOld := parseRavenVersion(oldTag)
	newVer, errNew := parseRavenVersion(newTag)
	if errOld != nil || errNew != nil {
		return fmt.Errorf(
			"unable to parse RavenDB versions from tags (old=%q err=%v, new=%q err=%v). "+
				"Use tags that start with '<major>.<minor>.<patch>...'",
			oldTag, errOld, newTag, errNew,
		)
	}
	if compareSemver(oldVer, newVer) == 1 {
		return fmt.Errorf(
			"downgrade is not allowed: %s (v%d.%d.%d) -> %s (v%d.%d.%d)",
			oldTag, oldVer.major, oldVer.minor, oldVer.patch,
			newTag, newVer.major, newVer.minor, newVer.patch,
		)
	}
	return nil
}

func isRavenRepo(image string) bool {
	return strings.HasPrefix(image, "ravendb/")
}

func hasDigest(image string) bool {
	return strings.Contains(image, "@sha256:")
}

func extractTag(image string) (string, bool) {
	if i := strings.Index(image, "@"); i != -1 {
		image = image[:i]
	}

	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")

	if lastColon == -1 || lastColon < lastSlash {
		return "", false
	}

	tag := image[lastColon+1:]
	if tag == "" {
		return "", false
	}
	return tag, true
}

func isFloatingTag(tag string) bool {
	return strings.Contains(tag, "latest")
}

func isUbuntuTag(tag string) bool {
	return strings.Contains(tag, "ubuntu.")
}

// parses the version at the start of the tag, e.g. "7.1.3" in: "7.1.3-ubuntu.22.04-x64"
var leadingSemverRE = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?`)

type semver struct {
	major int
	minor int
	patch int
}

func parseRavenVersion(tag string) (semver, error) {
	m := leadingSemverRE.FindStringSubmatch(tag)
	if len(m) == 0 {
		return semver{}, fmt.Errorf("no leading semver in tag %q", tag)
	}

	maj, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	pat := 0
	if len(m) > 3 && m[3] != "" {
		pat, _ = strconv.Atoi(m[3])
	}
	return semver{maj, min, pat}, nil
}

func compareSemver(a, b semver) int {
	if a.major != b.major {
		if a.major > b.major {
			return 1
		}
		return -1
	}

	if a.minor != b.minor {
		if a.minor > b.minor {
			return 1
		}
		return -1
	}

	if a.patch != b.patch {
		if a.patch > b.patch {
			return 1
		}
		return -1
	}
	return 0
}
