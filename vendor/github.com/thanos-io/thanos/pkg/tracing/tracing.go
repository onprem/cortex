// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package tracing

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"github.com/thanos-io/objstore"
)

const (
	// ForceTracingBaggageKey is a request header name that forces tracing sampling.
	ForceTracingBaggageKey = "X-Thanos-Force-Tracing"

	// traceIdResponseHeader is a response header name that stores trace ID.
	traceIDResponseHeader = "X-Thanos-Trace-Id"
)

// Aliases to avoid spreading opentracing package to Thanos code.

type Tag = opentracing.Tag
type Tags = opentracing.Tags
type Span = opentracing.Span

type contextKey struct{}

var tracerKey = contextKey{}

// Tracer interface to provide GetTraceIDFromSpanContext method.
type Tracer interface {
	GetTraceIDFromSpanContext(ctx opentracing.SpanContext) (string, bool)
}

// ContextWithTracer returns a new `context.Context` that holds a reference to given opentracing.Tracer.
func ContextWithTracer(ctx context.Context, tracer opentracing.Tracer) context.Context {
	return context.WithValue(
		objstore.ContextWithTracer(ctx, tracer),
		tracerKey,
		tracer,
	)
}

// tracerFromContext extracts opentracing.Tracer from the given context.
func tracerFromContext(ctx context.Context) opentracing.Tracer {
	val := ctx.Value(tracerKey)
	if sp, ok := val.(opentracing.Tracer); ok {
		return sp
	}
	return nil
}

// CopyTraceContext copies the necessary trace context from given source context to target context.
func CopyTraceContext(trgt, src context.Context) context.Context {
	ctx := ContextWithTracer(trgt, tracerFromContext(src))
	if parentSpan := opentracing.SpanFromContext(src); parentSpan != nil {
		ctx = opentracing.ContextWithSpan(ctx, parentSpan)
	}
	return ctx
}

// StartSpan starts and returns span with `operationName` and hooking as child to a span found within given context if any.
// It uses opentracing.Tracer propagated in context. If no found, it uses noop tracer without notification.
func StartSpan(ctx context.Context, operationName string, opts ...opentracing.StartSpanOption) (Span, context.Context) {
	tracer := tracerFromContext(ctx)
	if tracer == nil {
		// No tracing found, return noop span.
		return opentracing.NoopTracer{}.StartSpan(operationName), ctx
	}

	var span Span
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		opts = append(opts, opentracing.ChildOf(parentSpan.Context()))
	}
	span = tracer.StartSpan(operationName, opts...)
	return span, opentracing.ContextWithSpan(ctx, span)
}

// DoInSpan executes function doFn inside new span with `operationName` name and hooking as child to a span found within given context if any.
// It uses opentracing.Tracer propagated in context. If no found, it uses noop tracer notification.
func DoInSpan(ctx context.Context, operationName string, doFn func(context.Context), opts ...opentracing.StartSpanOption) {
	span, newCtx := StartSpan(ctx, operationName, opts...)
	defer span.Finish()
	doFn(newCtx)
}

// DoWithSpan executes function doFn inside new span with `operationName` name and hooking as child to a span found within given context if any.
// It uses opentracing.Tracer propagated in context. If no found, it uses noop tracer notification.
func DoWithSpan(ctx context.Context, operationName string, doFn func(context.Context, Span), opts ...opentracing.StartSpanOption) {
	span, newCtx := StartSpan(ctx, operationName, opts...)
	defer span.Finish()
	doFn(newCtx, span)
}
