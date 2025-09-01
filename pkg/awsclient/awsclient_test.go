package awsclient_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/irenedo/pia-operator/pkg/awsclient"
	mocks "github.com/irenedo/pia-operator/pkg/awsclient/mocks"
)

var _ = Describe("AWSClient", func() {
	var (
		ctx            context.Context
		mockClient     *mocks.MockAWSClient
		serviceAccount *corev1.ServiceAccount
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = &mocks.MockAWSClient{}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-sa",
				Namespace:   "default",
				Annotations: make(map[string]string),
			},
		}
	})

	Describe("CreatePodIdentityAssociation", func() {
		Context("when creating association successfully", func() {
			It("should return association ID with assume role", func() {
				expectedAssociationID := "a-12345"
				roleArn := "arn:aws:iam::123456789012:role/test-role"
				assumeRoleArn := "arn:aws:iam::123456789012:role/assume-role"

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleArn, assumeRoleArn).Return(expectedAssociationID, nil)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleArn, assumeRoleArn)

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return association ID without assume role", func() {
				expectedAssociationID := "a-12345"
				roleArn := "arn:aws:iam::123456789012:role/test-role"

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle empty assume role as empty string", func() {
				expectedAssociationID := "a-67890"
				roleArn := "arn:aws:iam::123456789012:role/base-role"
				emptyAssumeRole := ""

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleArn, emptyAssumeRole).Return(expectedAssociationID, nil)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleArn, emptyAssumeRole)

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle service account with existing annotations", func() {
				expectedAssociationID := "a-54321"
				roleArn := "arn:aws:iam::123456789012:role/annotated-role"
				serviceAccountWithAnnotations := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "annotated-sa",
						Namespace: "kube-system",
						Annotations: map[string]string{
							"existing-annotation": "existing-value",
						},
					},
				}

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccountWithAnnotations, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccountWithAnnotations, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle service account with nil annotations", func() {
				expectedAssociationID := "a-11111"
				roleArn := "arn:aws:iam::123456789012:role/nil-annotations-role"
				serviceAccountNilAnnotations := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "nil-annotations-sa",
						Namespace:   "production",
						Annotations: nil,
					},
				}

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccountNilAnnotations, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccountNilAnnotations, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle different namespace", func() {
				expectedAssociationID := "a-99999"
				roleArn := "arn:aws:iam::123456789012:role/different-ns-role"
				differentNamespaceSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "different-ns-sa",
						Namespace:   "monitoring",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("CreatePodIdentityAssociation", ctx, differentNamespaceSA, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, differentNamespaceSA, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when creation fails", func() {
			It("should return generic AWS API error", func() {
				roleArn := "arn:aws:iam::123456789012:role/test-role"
				expectedError := errors.New("AWS API error")

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", expectedError)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(expectedError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when association already exists", func() {
				roleArn := "arn:aws:iam::123456789012:role/existing-role"
				alreadyExistsError := errors.New("ResourceInUseException: Association already exists")

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", alreadyExistsError)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(alreadyExistsError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when cluster not found", func() {
				roleArn := "arn:aws:iam::123456789012:role/test-role"
				clusterNotFoundError := &types.ResourceNotFoundException{
					Message: aws.String("Cluster not found"),
				}

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", clusterNotFoundError)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(clusterNotFoundError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when role ARN is invalid", func() {
				invalidRoleArn := "invalid-role-arn"
				invalidParameterError := errors.New("InvalidParameterException: Invalid role ARN")

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, invalidRoleArn, "").Return("", invalidParameterError)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, invalidRoleArn, "")

				Expect(err).To(Equal(invalidParameterError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when assume role ARN is invalid", func() {
				roleArn := "arn:aws:iam::123456789012:role/valid-role"
				invalidAssumeRoleArn := "invalid-assume-role-arn"
				invalidAssumeRoleError := errors.New("InvalidParameterException: Invalid target role ARN")

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleArn, invalidAssumeRoleArn).Return("", invalidAssumeRoleError)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleArn, invalidAssumeRoleArn)

				Expect(err).To(Equal(invalidAssumeRoleError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when unauthorized", func() {
				roleArn := "arn:aws:iam::123456789012:role/test-role"
				unauthorizedError := errors.New("UnauthorizedOperation: Access denied")

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", unauthorizedError)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(unauthorizedError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when service limit exceeded", func() {
				roleArn := "arn:aws:iam::123456789012:role/test-role"
				serviceLimitError := errors.New("ServiceLimitExceededException: Service limit exceeded")

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", serviceLimitError)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(serviceLimitError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when context is cancelled", func() {
				roleArn := "arn:aws:iam::123456789012:role/test-role"
				cancelledError := context.Canceled

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", cancelledError)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(cancelledError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when context times out", func() {
				roleArn := "arn:aws:iam::123456789012:role/test-role"
				timeoutError := context.DeadlineExceeded

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", timeoutError)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(timeoutError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when handling edge cases", func() {
			It("should handle very long service account name", func() {
				expectedAssociationID := "a-long-name"
				roleArn := "arn:aws:iam::123456789012:role/test-role"
				longNameSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "very-long-service-account-name-that-might-cause-issues-with-token-generation-or-other-limits",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("CreatePodIdentityAssociation", ctx, longNameSA, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, longNameSA, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle service account with special characters in name", func() {
				expectedAssociationID := "a-special-chars"
				roleArn := "arn:aws:iam::123456789012:role/test-role"
				specialCharsSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-sa.with-dots_and-dashes",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("CreatePodIdentityAssociation", ctx, specialCharsSA, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, specialCharsSA, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle cross-account role ARN", func() {
				expectedAssociationID := "a-cross-account"
				crossAccountRoleArn := "arn:aws:iam::987654321098:role/cross-account-role"

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, crossAccountRoleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, crossAccountRoleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle cross-account assume role ARN", func() {
				expectedAssociationID := "a-cross-account-assume"
				roleArn := "arn:aws:iam::123456789012:role/base-role"
				crossAccountAssumeRoleArn := "arn:aws:iam::987654321098:role/cross-account-assume-role"

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleArn, crossAccountAssumeRoleArn).Return(expectedAssociationID, nil)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleArn, crossAccountAssumeRoleArn)

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle role with path in ARN", func() {
				expectedAssociationID := "a-role-with-path"
				roleWithPathArn := "arn:aws:iam::123456789012:role/path/to/role/test-role"

				mockClient.On("CreatePodIdentityAssociation", ctx, serviceAccount, roleWithPathArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.CreatePodIdentityAssociation(ctx, serviceAccount, roleWithPathArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("UpdatePodIdentityAssociation", func() {
		Context("when updating association successfully", func() {
			It("should return association ID with assume role", func() {
				expectedAssociationID := "a-12345"
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				assumeRoleArn := "arn:aws:iam::123456789012:role/updated-assume-role"

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, assumeRoleArn).Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, assumeRoleArn)

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return association ID without assume role", func() {
				expectedAssociationID := "a-54321"
				roleArn := "arn:aws:iam::123456789012:role/updated-role"

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle empty assume role as empty string", func() {
				expectedAssociationID := "a-67890"
				roleArn := "arn:aws:iam::123456789012:role/base-updated-role"
				emptyAssumeRole := ""

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, emptyAssumeRole).Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, emptyAssumeRole)

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should update association with existing association ID in annotations", func() {
				expectedAssociationID := "a-existing-id"
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				serviceAccountWithID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-with-id",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-existing-id",
						},
					},
				}

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccountWithID, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccountWithID, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should update association by finding it when no association ID in annotations", func() {
				expectedAssociationID := "a-found-association"
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				serviceAccountNoID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "sa-no-id",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccountNoID, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccountNoID, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle different namespace update", func() {
				expectedAssociationID := "a-different-ns"
				roleArn := "arn:aws:iam::123456789012:role/updated-ns-role"
				differentNamespaceSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "updated-ns-sa",
						Namespace:   "production",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("UpdatePodIdentityAssociation", ctx, differentNamespaceSA, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, differentNamespaceSA, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle updating to cross-account role", func() {
				expectedAssociationID := "a-cross-account-update"
				crossAccountRoleArn := "arn:aws:iam::987654321098:role/cross-account-updated-role"

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, crossAccountRoleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, crossAccountRoleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle updating with cross-account assume role", func() {
				expectedAssociationID := "a-cross-account-assume-update"
				roleArn := "arn:aws:iam::123456789012:role/base-role"
				crossAccountAssumeRoleArn := "arn:aws:iam::987654321098:role/cross-account-updated-assume-role"

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, crossAccountAssumeRoleArn).Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, crossAccountAssumeRoleArn)

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when update fails", func() {
			It("should return generic update error", func() {
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				expectedError := errors.New("update failed")

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", expectedError)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(expectedError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when association not found", func() {
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				notFoundError := &types.ResourceNotFoundException{
					Message: aws.String("Association not found"),
				}

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", notFoundError)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(notFoundError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when cluster not found", func() {
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				clusterNotFoundError := &types.ResourceNotFoundException{
					Message: aws.String("Cluster not found"),
				}

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", clusterNotFoundError)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(clusterNotFoundError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when role ARN is invalid", func() {
				invalidRoleArn := "invalid-updated-role-arn"
				invalidParameterError := errors.New("InvalidParameterException: Invalid role ARN")

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, invalidRoleArn, "").Return("", invalidParameterError)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, invalidRoleArn, "")

				Expect(err).To(Equal(invalidParameterError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when assume role ARN is invalid", func() {
				roleArn := "arn:aws:iam::123456789012:role/valid-updated-role"
				invalidAssumeRoleArn := "invalid-updated-assume-role-arn"
				invalidAssumeRoleError := errors.New("InvalidParameterException: Invalid target role ARN")

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, invalidAssumeRoleArn).Return("", invalidAssumeRoleError)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, invalidAssumeRoleArn)

				Expect(err).To(Equal(invalidAssumeRoleError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when unauthorized", func() {
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				unauthorizedError := errors.New("UnauthorizedOperation: Access denied")

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", unauthorizedError)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(unauthorizedError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when association is in invalid state", func() {
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				invalidStateError := errors.New("InvalidRequestException: Association is in invalid state")

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", invalidStateError)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(invalidStateError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when context is cancelled", func() {
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				cancelledError := context.Canceled

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", cancelledError)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(cancelledError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when context times out", func() {
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				timeoutError := context.DeadlineExceeded

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", timeoutError)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(timeoutError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when service account lookup fails", func() {
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				serviceAccountNoID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "sa-lookup-fail",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}
				lookupError := errors.New("failed to find existing association")

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccountNoID, roleArn, "").Return("", lookupError)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccountNoID, roleArn, "")

				Expect(err).To(Equal(lookupError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when handling edge cases", func() {
			It("should handle updating association with very long service account name", func() {
				expectedAssociationID := "a-long-name-update"
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				longNameSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "very-long-service-account-name-that-might-cause-issues-with-token-generation-or-other-limits-update",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("UpdatePodIdentityAssociation", ctx, longNameSA, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, longNameSA, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle updating association with special characters in service account name", func() {
				expectedAssociationID := "a-special-chars-update"
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				specialCharsSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "update-sa.with-dots_and-dashes",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("UpdatePodIdentityAssociation", ctx, specialCharsSA, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, specialCharsSA, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle updating to role with path in ARN", func() {
				expectedAssociationID := "a-role-path-update"
				roleWithPathArn := "arn:aws:iam::123456789012:role/path/to/updated/role/test-role"

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleWithPathArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleWithPathArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle updating association with empty association ID annotation", func() {
				expectedAssociationID := "a-empty-id-update"
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				serviceAccountEmptyID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-empty-id",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "",
						},
					},
				}

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccountEmptyID, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccountEmptyID, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle concurrent update scenario", func() {
				roleArn := "arn:aws:iam::123456789012:role/concurrent-updated-role"
				concurrentError := errors.New("ConflictException: Association is being modified")

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccount, roleArn, "").Return("", concurrentError)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccount, roleArn, "")

				Expect(err).To(Equal(concurrentError))
				Expect(associationID).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle service account with nil annotations map", func() {
				expectedAssociationID := "a-nil-annotations-update"
				roleArn := "arn:aws:iam::123456789012:role/updated-role"
				serviceAccountNilAnnotations := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "sa-nil-annotations-update",
						Namespace:   "staging",
						Annotations: nil,
					},
				}

				mockClient.On("UpdatePodIdentityAssociation", ctx, serviceAccountNilAnnotations, roleArn, "").Return(expectedAssociationID, nil)

				associationID, err := mockClient.UpdatePodIdentityAssociation(ctx, serviceAccountNilAnnotations, roleArn, "")

				Expect(err).ToNot(HaveOccurred())
				Expect(associationID).To(Equal(expectedAssociationID))
				mockClient.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("DeletePodIdentityAssociation", func() {
		Context("when deleting association successfully", func() {
			It("should successfully delete association with existing association ID", func() {
				serviceAccountWithID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-with-id",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-12345",
						},
					},
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccountWithID).Return(nil)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccountWithID)

				Expect(err).ToNot(HaveOccurred())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should successfully delete association by finding it when no association ID", func() {
				serviceAccountNoID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "sa-no-id",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccountNoID).Return(nil)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccountNoID)

				Expect(err).ToNot(HaveOccurred())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should successfully delete association from different namespace", func() {
				differentNamespaceSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "different-ns-sa",
						Namespace: "kube-system",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-different-ns",
						},
					},
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, differentNamespaceSA).Return(nil)

				err := mockClient.DeletePodIdentityAssociation(ctx, differentNamespaceSA)

				Expect(err).ToNot(HaveOccurred())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should successfully delete association with empty association ID annotation", func() {
				serviceAccountEmptyID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-empty-id",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "",
						},
					},
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccountEmptyID).Return(nil)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccountEmptyID)

				Expect(err).ToNot(HaveOccurred())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should successfully delete association with nil annotations", func() {
				serviceAccountNilAnnotations := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "sa-nil-annotations",
						Namespace:   "production",
						Annotations: nil,
					},
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccountNilAnnotations).Return(nil)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccountNilAnnotations)

				Expect(err).ToNot(HaveOccurred())
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when association not found", func() {
			It("should not return error when association not found by ID", func() {
				notFoundError := &types.ResourceNotFoundException{
					Message: aws.String("Association not found"),
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccount).Return(notFoundError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(notFoundError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should not return error when association lookup fails with not found", func() {
				serviceAccountNoID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "sa-not-found",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}
				notFoundError := errors.New("Pod Identity Association not found for ServiceAccount")

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccountNoID).Return(notFoundError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccountNoID)

				Expect(err).To(Equal(notFoundError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle already deleted association gracefully", func() {
				alreadyDeletedError := &types.ResourceNotFoundException{
					Message: aws.String("PodIdentityAssociation not found"),
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccount).Return(alreadyDeletedError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(alreadyDeletedError))
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when deletion fails", func() {
			It("should return generic deletion error", func() {
				expectedError := errors.New("delete failed")

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccount).Return(expectedError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(expectedError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when cluster not found", func() {
				clusterNotFoundError := &types.ResourceNotFoundException{
					Message: aws.String("Cluster not found"),
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccount).Return(clusterNotFoundError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(clusterNotFoundError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when unauthorized", func() {
				unauthorizedError := errors.New("UnauthorizedOperation: Access denied")

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccount).Return(unauthorizedError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(unauthorizedError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when association is in invalid state", func() {
				invalidStateError := errors.New("InvalidRequestException: Association cannot be deleted in current state")

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccount).Return(invalidStateError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(invalidStateError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when context is cancelled", func() {
				cancelledError := context.Canceled

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccount).Return(cancelledError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(cancelledError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when context times out", func() {
				timeoutError := context.DeadlineExceeded

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccount).Return(timeoutError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(timeoutError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when association lookup fails with other error", func() {
				serviceAccountNoID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "sa-lookup-error",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}
				lookupError := errors.New("failed to list associations")

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccountNoID).Return(lookupError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccountNoID)

				Expect(err).To(Equal(lookupError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error for invalid association ID", func() {
				serviceAccountInvalidID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-invalid-id",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "invalid-id-format",
						},
					},
				}
				invalidIDError := errors.New("InvalidParameterException: Invalid association ID")

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccountInvalidID).Return(invalidIDError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccountInvalidID)

				Expect(err).To(Equal(invalidIDError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when AWS service is unavailable", func() {
				serviceUnavailableError := errors.New("ServiceUnavailableException: Service temporarily unavailable")

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccount).Return(serviceUnavailableError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(serviceUnavailableError))
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when handling edge cases", func() {
			It("should handle deletion with very long service account name", func() {
				longNameSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "very-long-service-account-name-that-might-cause-issues-with-token-generation-or-other-limits-delete",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, longNameSA).Return(nil)

				err := mockClient.DeletePodIdentityAssociation(ctx, longNameSA)

				Expect(err).ToNot(HaveOccurred())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle deletion with special characters in service account name", func() {
				specialCharsSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "delete-sa.with-dots_and-dashes",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, specialCharsSA).Return(nil)

				err := mockClient.DeletePodIdentityAssociation(ctx, specialCharsSA)

				Expect(err).ToNot(HaveOccurred())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle concurrent deletion scenario", func() {
				concurrentError := errors.New("ConflictException: Association is being modified")

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccount).Return(concurrentError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(concurrentError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle deletion with extra annotations", func() {
				serviceAccountWithExtras := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-with-extras",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id":          "a-12345",
							"kubectl.kubernetes.io/last-applied-configuration": "{}",
							"custom-annotation":                                "custom-value",
						},
					},
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccountWithExtras).Return(nil)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccountWithExtras)

				Expect(err).ToNot(HaveOccurred())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle deletion when association ID has whitespace", func() {
				serviceAccountWhitespaceID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-whitespace-id",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "  a-12345  ",
						},
					},
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccountWhitespaceID).Return(nil)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccountWhitespaceID)

				Expect(err).ToNot(HaveOccurred())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle rate limiting error", func() {
				rateLimitError := errors.New("TooManyRequestsException: Rate limit exceeded")

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccount).Return(rateLimitError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(rateLimitError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle deletion when association is being created", func() {
				associationCreatingError := errors.New("InvalidRequestException: Association is being created")

				mockClient.On("DeletePodIdentityAssociation", ctx, serviceAccount).Return(associationCreatingError)

				err := mockClient.DeletePodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(associationCreatingError))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle deletion in different region context", func() {
				differentRegionSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "different-region-sa",
						Namespace: "kube-system",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-region-12345",
						},
					},
				}

				mockClient.On("DeletePodIdentityAssociation", ctx, differentRegionSA).Return(nil)

				err := mockClient.DeletePodIdentityAssociation(ctx, differentRegionSA)

				Expect(err).ToNot(HaveOccurred())
				mockClient.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("GetPodIdentityAssociation", func() {
		Context("when getting association successfully", func() {
			It("should return association by existing association ID", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-12345",
					ClusterName:        "test-cluster",
					Namespace:          "default",
					ServiceAccountName: "test-sa",
					RoleArn:            "arn:aws:iam::123456789012:role/test-role",
					Status:             "ACTIVE",
				}
				serviceAccountWithID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-12345",
						},
					},
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccountWithID).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccountWithID)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return association by finding it when no association ID", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-found-12345",
					ClusterName:        "test-cluster",
					Namespace:          "default",
					ServiceAccountName: "test-sa",
					RoleArn:            "arn:aws:iam::123456789012:role/found-role",
					Status:             "ACTIVE",
				}
				serviceAccountNoID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-sa",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccountNoID).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccountNoID)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return association with complete metadata", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-complete-12345",
					ClusterName:        "production-cluster",
					Namespace:          "kube-system",
					ServiceAccountName: "complete-sa",
					RoleArn:            "arn:aws:iam::123456789012:role/complete-role",
					AssumeRolePolicy:   "assume-role-policy-document",
					Tags: map[string]string{
						"Environment": "production",
						"Team":        "platform",
					},
					Status:     "ACTIVE",
					CreatedAt:  aws.String("2023-01-01T00:00:00Z"),
					ModifiedAt: aws.String("2023-01-01T12:00:00Z"),
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				Expect(association.Tags["Environment"]).To(Equal("production"))
				Expect(association.Tags["Team"]).To(Equal("platform"))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return association from different namespace", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-different-ns",
					ClusterName:        "test-cluster",
					Namespace:          "monitoring",
					ServiceAccountName: "monitoring-sa",
					RoleArn:            "arn:aws:iam::123456789012:role/monitoring-role",
					Status:             "ACTIVE",
				}
				differentNamespaceSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "monitoring-sa",
						Namespace: "monitoring",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-different-ns",
						},
					},
				}

				mockClient.On("GetPodIdentityAssociation", ctx, differentNamespaceSA).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, differentNamespaceSA)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return association with cross-account role", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-cross-account",
					ClusterName:        "test-cluster",
					Namespace:          "default",
					ServiceAccountName: "cross-account-sa",
					RoleArn:            "arn:aws:iam::987654321098:role/cross-account-role",
					Status:             "ACTIVE",
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return association with empty association ID annotation", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-empty-id-lookup",
					ClusterName:        "test-cluster",
					Namespace:          "default",
					ServiceAccountName: "empty-id-sa",
					RoleArn:            "arn:aws:iam::123456789012:role/empty-id-role",
					Status:             "ACTIVE",
				}
				serviceAccountEmptyID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "empty-id-sa",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "",
						},
					},
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccountEmptyID).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccountEmptyID)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return association with nil annotations", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-nil-annotations",
					ClusterName:        "test-cluster",
					Namespace:          "production",
					ServiceAccountName: "nil-annotations-sa",
					RoleArn:            "arn:aws:iam::123456789012:role/nil-annotations-role",
					Status:             "ACTIVE",
				}
				serviceAccountNilAnnotations := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "nil-annotations-sa",
						Namespace:   "production",
						Annotations: nil,
					},
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccountNilAnnotations).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccountNilAnnotations)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when association not found", func() {
			It("should return error when association not found by ID", func() {
				expectedError := errors.New("Pod Identity Association not found")
				serviceAccountWithID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "not-found-sa",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-not-found",
						},
					},
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccountWithID).Return(nil, expectedError)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccountWithID)

				Expect(err).To(Equal(expectedError))
				Expect(association).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when association lookup fails", func() {
				expectedError := errors.New("Pod Identity Association not found for ServiceAccount default/lookup-fail-sa")
				serviceAccountNoID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "lookup-fail-sa",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccountNoID).Return(nil, expectedError)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccountNoID)

				Expect(err).To(Equal(expectedError))
				Expect(association).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return ResourceNotFoundException when association does not exist", func() {
				notFoundError := &types.ResourceNotFoundException{
					Message: aws.String("Association not found"),
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(nil, notFoundError)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(notFoundError))
				Expect(association).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when get operation fails", func() {
			It("should return generic AWS API error", func() {
				expectedError := errors.New("AWS API error")

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(nil, expectedError)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(expectedError))
				Expect(association).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when cluster not found", func() {
				clusterNotFoundError := &types.ResourceNotFoundException{
					Message: aws.String("Cluster not found"),
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(nil, clusterNotFoundError)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(clusterNotFoundError))
				Expect(association).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when unauthorized", func() {
				unauthorizedError := errors.New("UnauthorizedOperation: Access denied")

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(nil, unauthorizedError)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(unauthorizedError))
				Expect(association).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when invalid association ID", func() {
				serviceAccountInvalidID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-id-sa",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "invalid-format",
						},
					},
				}
				invalidIDError := errors.New("InvalidParameterException: Invalid association ID")

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccountInvalidID).Return(nil, invalidIDError)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccountInvalidID)

				Expect(err).To(Equal(invalidIDError))
				Expect(association).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when context is cancelled", func() {
				cancelledError := context.Canceled

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(nil, cancelledError)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(cancelledError))
				Expect(association).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when context times out", func() {
				timeoutError := context.DeadlineExceeded

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(nil, timeoutError)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(timeoutError))
				Expect(association).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when list associations fails during lookup", func() {
				serviceAccountNoID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "list-fail-sa",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}
				listError := errors.New("failed to list Pod Identity Associations")

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccountNoID).Return(nil, listError)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccountNoID)

				Expect(err).To(Equal(listError))
				Expect(association).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when AWS service is unavailable", func() {
				serviceUnavailableError := errors.New("ServiceUnavailableException: Service temporarily unavailable")

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(nil, serviceUnavailableError)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(serviceUnavailableError))
				Expect(association).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when rate limited", func() {
				rateLimitError := errors.New("TooManyRequestsException: Rate limit exceeded")

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(nil, rateLimitError)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).To(Equal(rateLimitError))
				Expect(association).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when handling edge cases", func() {
			It("should handle service account with very long name", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-long-name",
					ClusterName:        "test-cluster",
					Namespace:          "default",
					ServiceAccountName: "very-long-service-account-name-that-might-cause-issues-with-token-generation-or-other-limits-get",
					RoleArn:            "arn:aws:iam::123456789012:role/long-name-role",
					Status:             "ACTIVE",
				}
				longNameSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "very-long-service-account-name-that-might-cause-issues-with-token-generation-or-other-limits-get",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("GetPodIdentityAssociation", ctx, longNameSA).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, longNameSA)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle service account with special characters in name", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-special-chars",
					ClusterName:        "test-cluster",
					Namespace:          "default",
					ServiceAccountName: "get-sa.with-dots_and-dashes",
					RoleArn:            "arn:aws:iam::123456789012:role/special-chars-role",
					Status:             "ACTIVE",
				}
				specialCharsSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "get-sa.with-dots_and-dashes",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("GetPodIdentityAssociation", ctx, specialCharsSA).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, specialCharsSA)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle association with role that has path", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-role-path",
					ClusterName:        "test-cluster",
					Namespace:          "default",
					ServiceAccountName: "role-path-sa",
					RoleArn:            "arn:aws:iam::123456789012:role/path/to/role/test-role",
					Status:             "ACTIVE",
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle association in different states", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-creating-state",
					ClusterName:        "test-cluster",
					Namespace:          "default",
					ServiceAccountName: "creating-sa",
					RoleArn:            "arn:aws:iam::123456789012:role/creating-role",
					Status:             "CREATING",
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				Expect(association.Status).To(Equal("CREATING"))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle association with empty tags", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-empty-tags",
					ClusterName:        "test-cluster",
					Namespace:          "default",
					ServiceAccountName: "empty-tags-sa",
					RoleArn:            "arn:aws:iam::123456789012:role/empty-tags-role",
					Tags:               make(map[string]string),
					Status:             "ACTIVE",
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				Expect(association.Tags).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle association with nil timestamps", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-nil-timestamps",
					ClusterName:        "test-cluster",
					Namespace:          "default",
					ServiceAccountName: "nil-timestamps-sa",
					RoleArn:            "arn:aws:iam::123456789012:role/nil-timestamps-role",
					Status:             "ACTIVE",
					CreatedAt:          nil,
					ModifiedAt:         nil,
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccount).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccount)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				Expect(association.CreatedAt).To(BeNil())
				Expect(association.ModifiedAt).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle association with whitespace in association ID annotation", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-whitespace-id",
					ClusterName:        "test-cluster",
					Namespace:          "default",
					ServiceAccountName: "whitespace-sa",
					RoleArn:            "arn:aws:iam::123456789012:role/whitespace-role",
					Status:             "ACTIVE",
				}
				serviceAccountWhitespace := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "whitespace-sa",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "  a-whitespace-id  ",
						},
					},
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccountWhitespace).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccountWhitespace)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle association with extra annotations", func() {
				expectedAssociation := &awsclient.PodIdentityAssociation{
					ID:                 "a-extra-annotations",
					ClusterName:        "test-cluster",
					Namespace:          "default",
					ServiceAccountName: "extra-annotations-sa",
					RoleArn:            "arn:aws:iam::123456789012:role/extra-annotations-role",
					Status:             "ACTIVE",
				}
				serviceAccountWithExtras := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "extra-annotations-sa",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id":          "a-extra-annotations",
							"kubectl.kubernetes.io/last-applied-configuration": "{}",
							"custom-annotation":                                "custom-value",
							"another-annotation":                               "another-value",
						},
					},
				}

				mockClient.On("GetPodIdentityAssociation", ctx, serviceAccountWithExtras).Return(expectedAssociation, nil)

				association, err := mockClient.GetPodIdentityAssociation(ctx, serviceAccountWithExtras)

				Expect(err).ToNot(HaveOccurred())
				Expect(association).To(Equal(expectedAssociation))
				mockClient.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("AssociationExists", func() {
		Context("when association exists", func() {
			It("should return true for service account with association ID", func() {
				serviceAccountWithID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-with-id",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-12345",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountWithID).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountWithID)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return true for service account found by lookup when no association ID", func() {
				serviceAccountNoID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "sa-no-id",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountNoID).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountNoID)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return true for association in different namespace", func() {
				differentNamespaceSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "different-ns-sa",
						Namespace: "kube-system",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-different-ns",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, differentNamespaceSA).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, differentNamespaceSA)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return true for service account with empty association ID annotation", func() {
				serviceAccountEmptyID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-empty-id",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountEmptyID).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountEmptyID)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return true for service account with nil annotations", func() {
				serviceAccountNilAnnotations := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "sa-nil-annotations",
						Namespace:   "production",
						Annotations: nil,
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountNilAnnotations).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountNilAnnotations)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return true for association in CREATING state", func() {
				serviceAccountCreating := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-creating",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-creating",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountCreating).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountCreating)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return true for association in ACTIVE state", func() {
				serviceAccountActive := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-active",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-active",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountActive).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountActive)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when association does not exist", func() {
			It("should return false for non-existent association by ID", func() {
				serviceAccountNotFound := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-not-found",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-not-found",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountNotFound).Return(false, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountNotFound)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return false for non-existent association by lookup", func() {
				serviceAccountNoID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "sa-lookup-not-found",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountNoID).Return(false, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountNoID)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return false when association deleted", func() {
				serviceAccountDeleted := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-deleted",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-deleted",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountDeleted).Return(false, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountDeleted)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return false when association not found in different namespace", func() {
				differentNamespaceSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "sa-different-ns-not-found",
						Namespace:   "monitoring",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("AssociationExists", ctx, differentNamespaceSA).Return(false, nil)

				exists, err := mockClient.AssociationExists(ctx, differentNamespaceSA)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return false for invalid association ID format", func() {
				serviceAccountInvalidID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-invalid-id",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "invalid-format",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountInvalidID).Return(false, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountInvalidID)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when check fails with errors", func() {
			It("should return error for generic API failure", func() {
				expectedError := errors.New("API error")

				mockClient.On("AssociationExists", ctx, serviceAccount).Return(false, expectedError)

				exists, err := mockClient.AssociationExists(ctx, serviceAccount)

				Expect(err).To(Equal(expectedError))
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when cluster not found", func() {
				clusterNotFoundError := &types.ResourceNotFoundException{
					Message: aws.String("Cluster not found"),
				}

				mockClient.On("AssociationExists", ctx, serviceAccount).Return(false, clusterNotFoundError)

				exists, err := mockClient.AssociationExists(ctx, serviceAccount)

				Expect(err).To(Equal(clusterNotFoundError))
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when unauthorized", func() {
				unauthorizedError := errors.New("UnauthorizedOperation: Access denied")

				mockClient.On("AssociationExists", ctx, serviceAccount).Return(false, unauthorizedError)

				exists, err := mockClient.AssociationExists(ctx, serviceAccount)

				Expect(err).To(Equal(unauthorizedError))
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when context is cancelled", func() {
				cancelledError := context.Canceled

				mockClient.On("AssociationExists", ctx, serviceAccount).Return(false, cancelledError)

				exists, err := mockClient.AssociationExists(ctx, serviceAccount)

				Expect(err).To(Equal(cancelledError))
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when context times out", func() {
				timeoutError := context.DeadlineExceeded

				mockClient.On("AssociationExists", ctx, serviceAccount).Return(false, timeoutError)

				exists, err := mockClient.AssociationExists(ctx, serviceAccount)

				Expect(err).To(Equal(timeoutError))
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when list associations fails during lookup", func() {
				serviceAccountNoID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "sa-list-fail",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}
				listError := errors.New("failed to list Pod Identity Associations")

				mockClient.On("AssociationExists", ctx, serviceAccountNoID).Return(false, listError)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountNoID)

				Expect(err).To(Equal(listError))
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when describe association fails", func() {
				serviceAccountWithID := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-describe-fail",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-describe-fail",
						},
					},
				}
				describeError := errors.New("failed to describe Pod Identity Association")

				mockClient.On("AssociationExists", ctx, serviceAccountWithID).Return(false, describeError)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountWithID)

				Expect(err).To(Equal(describeError))
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when AWS service is unavailable", func() {
				serviceUnavailableError := errors.New("ServiceUnavailableException: Service temporarily unavailable")

				mockClient.On("AssociationExists", ctx, serviceAccount).Return(false, serviceUnavailableError)

				exists, err := mockClient.AssociationExists(ctx, serviceAccount)

				Expect(err).To(Equal(serviceUnavailableError))
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when rate limited", func() {
				rateLimitError := errors.New("TooManyRequestsException: Rate limit exceeded")

				mockClient.On("AssociationExists", ctx, serviceAccount).Return(false, rateLimitError)

				exists, err := mockClient.AssociationExists(ctx, serviceAccount)

				Expect(err).To(Equal(rateLimitError))
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when invalid parameter provided", func() {
				serviceAccountInvalidParam := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-invalid-param",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-invalid-param",
						},
					},
				}
				invalidParamError := errors.New("InvalidParameterException: Invalid association ID")

				mockClient.On("AssociationExists", ctx, serviceAccountInvalidParam).Return(false, invalidParamError)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountInvalidParam)

				Expect(err).To(Equal(invalidParamError))
				Expect(exists).To(BeFalse())
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when handling edge cases", func() {
			It("should handle service account with very long name", func() {
				longNameSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "very-long-service-account-name-that-might-cause-issues-with-token-generation-or-other-limits-exists",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("AssociationExists", ctx, longNameSA).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, longNameSA)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle service account with special characters in name", func() {
				specialCharsSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "exists-sa.with-dots_and-dashes",
						Namespace:   "default",
						Annotations: make(map[string]string),
					},
				}

				mockClient.On("AssociationExists", ctx, specialCharsSA).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, specialCharsSA)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle association with whitespace in association ID annotation", func() {
				serviceAccountWhitespace := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-whitespace",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "  a-whitespace  ",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountWhitespace).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountWhitespace)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle service account with extra annotations", func() {
				serviceAccountWithExtras := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-with-extras",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id":          "a-with-extras",
							"kubectl.kubernetes.io/last-applied-configuration": "{}",
							"custom-annotation":                                "custom-value",
							"another-annotation":                               "another-value",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountWithExtras).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountWithExtras)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle checking existence multiple times for same service account", func() {
				serviceAccountRepeated := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-repeated",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-repeated",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountRepeated).Return(true, nil).Times(3)

				// First check
				exists1, err1 := mockClient.AssociationExists(ctx, serviceAccountRepeated)
				Expect(err1).ToNot(HaveOccurred())
				Expect(exists1).To(BeTrue())

				// Second check
				exists2, err2 := mockClient.AssociationExists(ctx, serviceAccountRepeated)
				Expect(err2).ToNot(HaveOccurred())
				Expect(exists2).To(BeTrue())

				// Third check
				exists3, err3 := mockClient.AssociationExists(ctx, serviceAccountRepeated)
				Expect(err3).ToNot(HaveOccurred())
				Expect(exists3).To(BeTrue())

				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle association in DELETING state", func() {
				serviceAccountDeleting := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-deleting",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-deleting",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountDeleting).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountDeleting)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle association in FAILED state", func() {
				serviceAccountFailed := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-failed",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-failed",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountFailed).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountFailed)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle concurrent existence checks", func() {
				serviceAccountConcurrent := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-concurrent",
						Namespace: "default",
						Annotations: map[string]string{
							"pia-operator.eks.aws.com/association-id": "a-concurrent",
						},
					},
				}

				mockClient.On("AssociationExists", ctx, serviceAccountConcurrent).Return(true, nil)

				exists, err := mockClient.AssociationExists(ctx, serviceAccountConcurrent)

				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
				mockClient.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("ListPodIdentityAssociations", func() {
		Context("when listing associations successfully", func() {
			It("should return list of associations with multiple entries", func() {
				expectedAssociations := []*awsclient.PodIdentityAssociation{
					{
						ID:                 "a-12345",
						ClusterName:        "test-cluster",
						Namespace:          "default",
						ServiceAccountName: "test-sa-1",
						RoleArn:            "arn:aws:iam::123456789012:role/test-role-1",
					},
					{
						ID:                 "a-67890",
						ClusterName:        "test-cluster",
						Namespace:          "kube-system",
						ServiceAccountName: "test-sa-2",
						RoleArn:            "arn:aws:iam::123456789012:role/test-role-2",
					},
				}

				mockClient.On("ListPodIdentityAssociations", ctx).Return(expectedAssociations, nil)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(associations).To(Equal(expectedAssociations))
				Expect(associations).To(HaveLen(2))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return single association", func() {
				expectedAssociations := []*awsclient.PodIdentityAssociation{
					{
						ID:                 "a-single",
						ClusterName:        "test-cluster",
						Namespace:          "default",
						ServiceAccountName: "single-sa",
						RoleArn:            "arn:aws:iam::123456789012:role/single-role",
					},
				}

				mockClient.On("ListPodIdentityAssociations", ctx).Return(expectedAssociations, nil)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(associations).To(Equal(expectedAssociations))
				Expect(associations).To(HaveLen(1))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return empty list when no associations exist", func() {
				expectedAssociations := []*awsclient.PodIdentityAssociation{}

				mockClient.On("ListPodIdentityAssociations", ctx).Return(expectedAssociations, nil)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(associations).To(Equal(expectedAssociations))
				Expect(associations).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return nil list when no associations exist", func() {
				var expectedAssociations []*awsclient.PodIdentityAssociation = nil

				mockClient.On("ListPodIdentityAssociations", ctx).Return(expectedAssociations, nil)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(associations).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return associations from different namespaces", func() {
				expectedAssociations := []*awsclient.PodIdentityAssociation{
					{
						ID:                 "a-default",
						ClusterName:        "test-cluster",
						Namespace:          "default",
						ServiceAccountName: "default-sa",
						RoleArn:            "arn:aws:iam::123456789012:role/default-role",
					},
					{
						ID:                 "a-kube-system",
						ClusterName:        "test-cluster",
						Namespace:          "kube-system",
						ServiceAccountName: "kube-system-sa",
						RoleArn:            "arn:aws:iam::123456789012:role/kube-system-role",
					},
					{
						ID:                 "a-monitoring",
						ClusterName:        "test-cluster",
						Namespace:          "monitoring",
						ServiceAccountName: "monitoring-sa",
						RoleArn:            "arn:aws:iam::123456789012:role/monitoring-role",
					},
				}

				mockClient.On("ListPodIdentityAssociations", ctx).Return(expectedAssociations, nil)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(associations).To(Equal(expectedAssociations))
				Expect(associations).To(HaveLen(3))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return associations with cross-account roles", func() {
				expectedAssociations := []*awsclient.PodIdentityAssociation{
					{
						ID:                 "a-cross-account-1",
						ClusterName:        "test-cluster",
						Namespace:          "default",
						ServiceAccountName: "cross-account-sa-1",
						RoleArn:            "arn:aws:iam::987654321098:role/cross-account-role-1",
					},
					{
						ID:                 "a-cross-account-2",
						ClusterName:        "test-cluster",
						Namespace:          "default",
						ServiceAccountName: "cross-account-sa-2",
						RoleArn:            "arn:aws:iam::111222333444:role/cross-account-role-2",
					},
				}

				mockClient.On("ListPodIdentityAssociations", ctx).Return(expectedAssociations, nil)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(associations).To(Equal(expectedAssociations))
				Expect(associations).To(HaveLen(2))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return large number of associations", func() {
				expectedAssociations := make([]*awsclient.PodIdentityAssociation, 100)
				for i := 0; i < 100; i++ {
					expectedAssociations[i] = &awsclient.PodIdentityAssociation{
						ID:                 fmt.Sprintf("a-%d", i),
						ClusterName:        "test-cluster",
						Namespace:          "default",
						ServiceAccountName: fmt.Sprintf("sa-%d", i),
						RoleArn:            fmt.Sprintf("arn:aws:iam::123456789012:role/role-%d", i),
					}
				}

				mockClient.On("ListPodIdentityAssociations", ctx).Return(expectedAssociations, nil)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(associations).To(Equal(expectedAssociations))
				Expect(associations).To(HaveLen(100))
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when listing fails", func() {
			It("should return generic list error", func() {
				expectedError := errors.New("list failed")

				mockClient.On("ListPodIdentityAssociations", ctx).Return(nil, expectedError)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).To(Equal(expectedError))
				Expect(associations).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when cluster not found", func() {
				clusterNotFoundError := &types.ResourceNotFoundException{
					Message: aws.String("Cluster not found"),
				}

				mockClient.On("ListPodIdentityAssociations", ctx).Return(nil, clusterNotFoundError)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).To(Equal(clusterNotFoundError))
				Expect(associations).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when unauthorized", func() {
				unauthorizedError := errors.New("UnauthorizedOperation: Access denied")

				mockClient.On("ListPodIdentityAssociations", ctx).Return(nil, unauthorizedError)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).To(Equal(unauthorizedError))
				Expect(associations).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when context is cancelled", func() {
				cancelledError := context.Canceled

				mockClient.On("ListPodIdentityAssociations", ctx).Return(nil, cancelledError)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).To(Equal(cancelledError))
				Expect(associations).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when context times out", func() {
				timeoutError := context.DeadlineExceeded

				mockClient.On("ListPodIdentityAssociations", ctx).Return(nil, timeoutError)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).To(Equal(timeoutError))
				Expect(associations).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when AWS service is unavailable", func() {
				serviceUnavailableError := errors.New("ServiceUnavailableException: Service temporarily unavailable")

				mockClient.On("ListPodIdentityAssociations", ctx).Return(nil, serviceUnavailableError)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).To(Equal(serviceUnavailableError))
				Expect(associations).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when rate limited", func() {
				rateLimitError := errors.New("TooManyRequestsException: Rate limit exceeded")

				mockClient.On("ListPodIdentityAssociations", ctx).Return(nil, rateLimitError)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).To(Equal(rateLimitError))
				Expect(associations).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error when invalid parameter provided", func() {
				invalidParamError := errors.New("InvalidParameterException: Invalid cluster name")

				mockClient.On("ListPodIdentityAssociations", ctx).Return(nil, invalidParamError)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).To(Equal(invalidParamError))
				Expect(associations).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should return error during pagination", func() {
				paginationError := errors.New("pagination failed")

				mockClient.On("ListPodIdentityAssociations", ctx).Return(nil, paginationError)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).To(Equal(paginationError))
				Expect(associations).To(BeNil())
				mockClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when handling different response scenarios", func() {
			It("should handle associations with complete metadata", func() {
				expectedAssociations := []*awsclient.PodIdentityAssociation{
					{
						ID:                 "a-complete",
						ClusterName:        "production-cluster",
						Namespace:          "default",
						ServiceAccountName: "complete-sa",
						RoleArn:            "arn:aws:iam::123456789012:role/complete-role",
						Tags: map[string]string{
							"Environment": "production",
							"Team":        "platform",
						},
						Status:     "ACTIVE",
						CreatedAt:  aws.String("2023-01-01T00:00:00Z"),
						ModifiedAt: aws.String("2023-01-01T12:00:00Z"),
					},
				}

				mockClient.On("ListPodIdentityAssociations", ctx).Return(expectedAssociations, nil)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(associations).To(Equal(expectedAssociations))
				Expect(associations[0].Tags["Environment"]).To(Equal("production"))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle associations with minimal metadata", func() {
				expectedAssociations := []*awsclient.PodIdentityAssociation{
					{
						ID:                 "a-minimal",
						ClusterName:        "test-cluster",
						Namespace:          "default",
						ServiceAccountName: "minimal-sa",
						RoleArn:            "arn:aws:iam::123456789012:role/minimal-role",
						Tags:               make(map[string]string),
					},
				}

				mockClient.On("ListPodIdentityAssociations", ctx).Return(expectedAssociations, nil)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(associations).To(Equal(expectedAssociations))
				Expect(associations[0].Tags).To(BeEmpty())
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle associations in different states", func() {
				expectedAssociations := []*awsclient.PodIdentityAssociation{
					{
						ID:                 "a-creating",
						ClusterName:        "test-cluster",
						Namespace:          "default",
						ServiceAccountName: "creating-sa",
						RoleArn:            "arn:aws:iam::123456789012:role/creating-role",
						Status:             "CREATING",
					},
					{
						ID:                 "a-active",
						ClusterName:        "test-cluster",
						Namespace:          "default",
						ServiceAccountName: "active-sa",
						RoleArn:            "arn:aws:iam::123456789012:role/active-role",
						Status:             "ACTIVE",
					},
					{
						ID:                 "a-deleting",
						ClusterName:        "test-cluster",
						Namespace:          "default",
						ServiceAccountName: "deleting-sa",
						RoleArn:            "arn:aws:iam::123456789012:role/deleting-role",
						Status:             "DELETING",
					},
				}

				mockClient.On("ListPodIdentityAssociations", ctx).Return(expectedAssociations, nil)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(associations).To(Equal(expectedAssociations))
				Expect(associations).To(HaveLen(3))
				mockClient.AssertExpectations(GinkgoT())
			})

			It("should handle mixed role ARNs with and without paths", func() {
				expectedAssociations := []*awsclient.PodIdentityAssociation{
					{
						ID:                 "a-no-path",
						ClusterName:        "test-cluster",
						Namespace:          "default",
						ServiceAccountName: "no-path-sa",
						RoleArn:            "arn:aws:iam::123456789012:role/no-path-role",
					},
					{
						ID:                 "a-with-path",
						ClusterName:        "test-cluster",
						Namespace:          "default",
						ServiceAccountName: "with-path-sa",
						RoleArn:            "arn:aws:iam::123456789012:role/path/to/role/with-path-role",
					},
				}

				mockClient.On("ListPodIdentityAssociations", ctx).Return(expectedAssociations, nil)

				associations, err := mockClient.ListPodIdentityAssociations(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(associations).To(Equal(expectedAssociations))
				Expect(associations).To(HaveLen(2))
				mockClient.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("PodIdentityAssociation struct", func() {
		It("should have correct fields", func() {
			association := &awsclient.PodIdentityAssociation{
				ID:                 "a-12345",
				ClusterName:        "test-cluster",
				Namespace:          "default",
				ServiceAccountName: "test-sa",
				RoleArn:            "arn:aws:iam::123456789012:role/test-role",
				AssumeRolePolicy:   "policy-document",
				Tags:               map[string]string{"key": "value"},
				Status:             "ACTIVE",
				CreatedAt:          aws.String("2023-01-01T00:00:00Z"),
				ModifiedAt:         aws.String("2023-01-01T00:00:00Z"),
			}

			Expect(association.ID).To(Equal("a-12345"))
			Expect(association.ClusterName).To(Equal("test-cluster"))
			Expect(association.Namespace).To(Equal("default"))
			Expect(association.ServiceAccountName).To(Equal("test-sa"))
			Expect(association.RoleArn).To(Equal("arn:aws:iam::123456789012:role/test-role"))
			Expect(association.Tags["key"]).To(Equal("value"))
			Expect(association.Status).To(Equal("ACTIVE"))
		})
	})

	Describe("AssociationStatus constants", func() {
		It("should have correct status values", func() {
			Expect(string(awsclient.AssociationStatusCreating)).To(Equal("CREATING"))
			Expect(string(awsclient.AssociationStatusActive)).To(Equal("ACTIVE"))
			Expect(string(awsclient.AssociationStatusDeleting)).To(Equal("DELETING"))
			Expect(string(awsclient.AssociationStatusFailed)).To(Equal("FAILED"))
		})
	})
})
