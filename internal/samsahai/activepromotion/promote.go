package activepromotion

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/queue"
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

func (c *controller) promoteActiveEnvironment(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	teamName := atpComp.Name
	targetNs := atpComp.Status.TargetNamespace
	prevNs := atpComp.Status.PreviousActiveNamespace

	if err := queue.DeletePreActiveQueue(c.client, targetNs); err != nil {
		return err
	}

	if err := c.ensureQueuePromotedToActive(teamName, targetNs); err != nil {
		return err
	}

	if err := c.ensureActiveEnvironmentPromoted(ctx, teamName, targetNs); err != nil {
		return err
	}

	if prevNs != "" && atpComp.Status.DestroyTime == nil {
		logger.Debug("previous active namespace destroyed time has been set",
			"team", teamName, "namespace", prevNs)
		destroyTime := metav1.Now().Add(atpComp.Spec.TearDownDuration.Duration)
		atpComp.Status.SetDestroyTime(metav1.Time{Time: destroyTime})
	}

	logger.Info("active environment has been promoted successfully",
		"team", teamName, "status", s2hv1beta1.ActivePromotionSuccess, "namespace", targetNs)
	atpComp.Status.SetResult(s2hv1beta1.ActivePromotionSuccess)
	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondResultCollected, corev1.ConditionTrue,
		"Result has been collected, promoted successfully")
	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondActivePromoted, corev1.ConditionTrue,
		"Active environment has been promoted")
	atpComp.SetState(s2hv1beta1.ActivePromotionDestroyingPreviousActive,
		"Destroying the previous active environment")

	if err := c.runPostActive(ctx, atpComp); err != nil {
		if s2herrors.IsLoadingConfiguration(err) {
			return s2herrors.ErrEnsureActivePromoted
		}
		return err
	}

	return nil
}

func (c *controller) ensureQueuePromotedToActive(teamName, ns string) error {
	q, err := queue.EnsurePromoteToActiveComponents(c.client, teamName, ns)
	if err != nil {
		return errors.Wrapf(err, "cannot ensure environment promoted to active components, namespace %s", ns)
	}

	if q.Status.State == s2hv1beta1.Finished {
		return nil
	}

	return s2herrors.ErrEnsureActivePromoted
}