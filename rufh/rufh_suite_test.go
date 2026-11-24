package rufh_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRufh(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rufh Suite")
}
