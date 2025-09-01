package errors_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pkgerrors "github.com/irenedo/pia-operator/pkg/errors"
	mocksclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ErrorHandler", func() {
	var (
		ctx            context.Context
		errorHandler   *pkgerrors.ErrorHandler
		serviceAccount *corev1.ServiceAccount
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Create a fake client
		client := mocksclient.NewClientBuilder().Build()
		logger := log.Log.WithName("test-error-handler")

		errorHandler = pkgerrors.NewErrorHandler(client, logger)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-sa",
				Namespace: "default",
			},
		}
	})

	Describe("NewErrorHandler", func() {
		It("should create a new error handler instance", func() {
			client := mocksclient.NewClientBuilder().Build()
			logger := log.Log.WithName("test")

			handler := pkgerrors.NewErrorHandler(client, logger)

			Expect(handler).NotTo(BeNil())
		})
	})

	Describe("ClassifyError", func() {
		Context("when error is nil", func() {
			It("should return ErrorRetryable", func() {
				classification := errorHandler.ClassifyError(nil)
				Expect(classification).To(Equal(pkgerrors.ErrorRetryable))
			})
		})

		Context("when error is permanent", func() {
			It("should return ErrorPermanent for NotFound error", func() {
				err := k8serrors.NewNotFound(schema.GroupResource{Group: "", Resource: "pods"}, "test")
				classification := errorHandler.ClassifyError(err)
				Expect(classification).To(Equal(pkgerrors.ErrorPermanent))
			})

			It("should return ErrorPermanent for Invalid error", func() {
				err := k8serrors.NewInvalid(schema.GroupKind{Group: "", Kind: "Pod"}, "test", nil)
				classification := errorHandler.ClassifyError(err)
				Expect(classification).To(Equal(pkgerrors.ErrorPermanent))
			})

			It("should return ErrorPermanent for BadRequest error", func() {
				err := k8serrors.NewBadRequest("bad request")
				classification := errorHandler.ClassifyError(err)
				Expect(classification).To(Equal(pkgerrors.ErrorPermanent))
			})
		})

		Context("when error is transient", func() {
			It("should return ErrorTransient for TooManyRequests error", func() {
				err := k8serrors.NewTooManyRequests("rate limited", 30)
				classification := errorHandler.ClassifyError(err)
				Expect(classification).To(Equal(pkgerrors.ErrorTransient))
			})

			It("should return ErrorTransient for ServiceUnavailable error", func() {
				err := k8serrors.NewServiceUnavailable("service unavailable")
				classification := errorHandler.ClassifyError(err)
				Expect(classification).To(Equal(pkgerrors.ErrorTransient))
			})

			It("should return ErrorTransient for Timeout error", func() {
				err := k8serrors.NewTimeoutError("timeout", 30)
				classification := errorHandler.ClassifyError(err)
				Expect(classification).To(Equal(pkgerrors.ErrorTransient))
			})
		})

		Context("when error is retryable", func() {
			It("should return ErrorRetryable for Conflict error", func() {
				err := k8serrors.NewConflict(schema.GroupResource{Group: "", Resource: "pods"}, "test", nil)
				classification := errorHandler.ClassifyError(err)
				Expect(classification).To(Equal(pkgerrors.ErrorRetryable))
			})
		})

		Context("when error is unknown", func() {
			It("should return ErrorTransient for unknown error types", func() {
				err := k8serrors.NewInternalError(errors.New("internal error"))
				classification := errorHandler.ClassifyError(err)
				Expect(classification).To(Equal(pkgerrors.ErrorTransient))
			})
		})
	})

	Describe("CalculateBackoff", func() {
		It("should return base delay for first retry", func() {
			delay := errorHandler.CalculateBackoff(0)
			Expect(delay).To(Equal(30 * time.Second))
		})

		It("should double the delay for each retry", func() {
			delay1 := errorHandler.CalculateBackoff(1)
			delay2 := errorHandler.CalculateBackoff(2)
			delay3 := errorHandler.CalculateBackoff(3)

			Expect(delay1).To(Equal(60 * time.Second))
			Expect(delay2).To(Equal(120 * time.Second))
			Expect(delay3).To(Equal(240 * time.Second))
		})

		It("should cap the delay at maximum", func() {
			delay := errorHandler.CalculateBackoff(10)
			Expect(delay).To(Equal(5 * time.Minute))
		})
	})

	Describe("GetRetryCount and SetRetryCount", func() {
		It("should return 0 for new service account", func() {
			count := errorHandler.GetRetryCount(serviceAccount)
			Expect(count).To(Equal(0))
		})

		It("should set and get retry count correctly", func() {
			errorHandler.SetRetryCount(ctx, serviceAccount, 3)
			count := errorHandler.GetRetryCount(serviceAccount)
			Expect(count).To(Equal(3))
		})

		It("should use namespace/name as key", func() {
			sa1 := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa-1",
					Namespace: "default",
				},
			}
			sa2 := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa-2",
					Namespace: "default",
				},
			}

			errorHandler.SetRetryCount(ctx, sa1, 2)
			errorHandler.SetRetryCount(ctx, sa2, 4)

			count1 := errorHandler.GetRetryCount(sa1)
			count2 := errorHandler.GetRetryCount(sa2)

			Expect(count1).To(Equal(2))
			Expect(count2).To(Equal(4))
		})
	})

	Describe("HandleError", func() {
		Context("when error is nil", func() {
			It("should return empty result", func() {
				result, err := errorHandler.HandleError(ctx, serviceAccount, nil, "test")
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("when error is permanent", func() {
			It("should not retry and return empty result", func() {
				notFoundErr := k8serrors.NewNotFound(schema.GroupResource{Group: "", Resource: "pods"}, "test")
				result, err := errorHandler.HandleError(ctx, serviceAccount, notFoundErr, "test")

				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("when error is retryable", func() {
			It("should retry immediately", func() {
				conflictErr := k8serrors.NewConflict(schema.GroupResource{Group: "", Resource: "pods"}, "test", nil)
				result, err := errorHandler.HandleError(ctx, serviceAccount, conflictErr, "test")

				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeTrue())
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("when error is transient", func() {
			It("should retry with backoff", func() {
				timeoutErr := k8serrors.NewTimeoutError("timeout", 30)
				result, err := errorHandler.HandleError(ctx, serviceAccount, timeoutErr, "test")

				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(30 * time.Second))

				// Check retry count was incremented
				retryCount := errorHandler.GetRetryCount(serviceAccount)
				Expect(retryCount).To(Equal(1))
			})

			It("should stop retrying after max attempts", func() {
				timeoutErr := k8serrors.NewTimeoutError("timeout", 30)

				// Set retry count to max
				errorHandler.SetRetryCount(ctx, serviceAccount, 5)

				result, err := errorHandler.HandleError(ctx, serviceAccount, timeoutErr, "test")

				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(5 * time.Minute))
			})
		})
	})

	Describe("HandleDeletionError", func() {
		Context("when error is nil", func() {
			It("should return empty result", func() {
				result, err := errorHandler.HandleDeletionError(ctx, serviceAccount, nil, "delete")
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("when error is permanent", func() {
			It("should continue deletion and return empty result", func() {
				notFoundErr := k8serrors.NewNotFound(schema.GroupResource{Group: "", Resource: "pods"}, "test")
				result, err := errorHandler.HandleDeletionError(ctx, serviceAccount, notFoundErr, "delete")

				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("when error is retryable", func() {
			It("should retry immediately", func() {
				conflictErr := k8serrors.NewConflict(schema.GroupResource{Group: "", Resource: "pods"}, "test", nil)
				result, err := errorHandler.HandleDeletionError(ctx, serviceAccount, conflictErr, "delete")

				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeTrue())
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("when error is transient", func() {
			It("should retry with backoff", func() {
				timeoutErr := k8serrors.NewTimeoutError("timeout", 30)
				result, err := errorHandler.HandleDeletionError(ctx, serviceAccount, timeoutErr, "delete")

				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(30 * time.Second))
			})

			It("should continue deletion after max attempts", func() {
				timeoutErr := k8serrors.NewTimeoutError("timeout", 30)

				// Set retry count to max
				errorHandler.SetRetryCount(ctx, serviceAccount, 5)

				result, err := errorHandler.HandleDeletionError(ctx, serviceAccount, timeoutErr, "delete")

				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})
		})
	})

	Describe("ResetRetryCount", func() {
		It("should reset retry count to 0", func() {
			errorHandler.SetRetryCount(ctx, serviceAccount, 5)
			errorHandler.ResetRetryCount(ctx, serviceAccount)

			count := errorHandler.GetRetryCount(serviceAccount)
			Expect(count).To(Equal(0))
		})
	})

	Describe("MarkSuccess", func() {
		It("should reset retry count", func() {
			errorHandler.SetRetryCount(ctx, serviceAccount, 3)
			errorHandler.MarkSuccess(ctx, serviceAccount, "operation successful")

			count := errorHandler.GetRetryCount(serviceAccount)
			Expect(count).To(Equal(0))
		})
	})
})
