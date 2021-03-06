package argo

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	"fmt"
	"time"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type AuditLogger struct {
	kIf       kubernetes.Interface
	component string
	ns        string
}

type EventInfo struct {
	Action   string
	Reason   string
	Username string
}

const (
	EventReasonStatusRefreshed = "StatusRefreshed"
	EventReasonResourceCreated = "ResourceCreated"
	EventReasonResourceUpdated = "ResourceUpdated"
	EventReasonResourceDeleted = "ResourceDeleted"
)

func (l *AuditLogger) logEvent(objMeta metav1.ObjectMeta, gvk schema.GroupVersionKind, info EventInfo, eventType string) {
	var message string
	if info.Username != "" {
		message = fmt.Sprintf("User %s executed action %s", info.Username, info.Action)
	} else {
		message = fmt.Sprintf("Unknown user executed action %s", info.Action)
	}
	logCtx := log.WithFields(log.Fields{
		"type":     eventType,
		"action":   info.Action,
		"reason":   info.Reason,
		"username": info.Username,
	})
	switch gvk.Kind {
	case "Application":
		logCtx = logCtx.WithField("application", objMeta.Name)
	case "AppProject":
		logCtx = logCtx.WithField("project", objMeta.Name)
	default:
		logCtx = logCtx.WithField("name", objMeta.Name)
	}
	t := metav1.Time{Time: time.Now()}
	event := v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%v.%x", objMeta.Name, t.UnixNano()),
		},
		Source: v1.EventSource{
			Component: l.component,
		},
		InvolvedObject: v1.ObjectReference{
			Kind:            gvk.Kind,
			Name:            objMeta.Name,
			Namespace:       objMeta.Namespace,
			ResourceVersion: objMeta.ResourceVersion,
			APIVersion:      gvk.Version,
			UID:             objMeta.UID,
		},
		FirstTimestamp: t,
		LastTimestamp:  t,
		Count:          1,
		Message:        message,
		Type:           eventType,
		Action:         info.Action,
		Reason:         info.Reason,
	}
	logCtx.Info(message)
	_, err := l.kIf.CoreV1().Events(l.ns).Create(&event)
	if err != nil {
		logCtx.Errorf("Unable to create audit event: %v", err)
		return
	}
}

func (l *AuditLogger) LogAppEvent(app *v1alpha1.Application, info EventInfo, eventType string) {
	l.logEvent(app.ObjectMeta, v1alpha1.ApplicationSchemaGroupVersionKind, info, eventType)
}

func (l *AuditLogger) LogAppProjEvent(proj *v1alpha1.AppProject, info EventInfo, eventType string) {
	l.logEvent(proj.ObjectMeta, v1alpha1.AppProjectSchemaGroupVersionKind, info, eventType)
}

func NewAuditLogger(ns string, kIf kubernetes.Interface, component string) *AuditLogger {
	return &AuditLogger{
		ns:        ns,
		kIf:       kIf,
		component: component,
	}
}
