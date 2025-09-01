package controller_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestServiceAccountController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ServiceAccount Controller Suite")
}
