package awsclient_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAWSClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWSClient Suite")
}
