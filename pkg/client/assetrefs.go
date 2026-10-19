package client

import (
	"context"
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/assetrefs"
	"github.com/geoffmilleraz/fathom/internal/core"
)

type AssetReferenceSnapshot = core.AssetReferenceSnapshot
type AssetReferenceQuery = core.AssetReferenceQuery
type AssetReferenceResult = core.AssetReferenceResult
type AssetObjectID = core.AssetObjectID

// UnityAssetReferenceOptions selects an explicit local producer receipt. It
// cannot choose a filesystem root; the client's registered service owns that.
// Preparation/query and the explicit CLI reader are read-only; no durable store
// importer or caller-prepared snapshot can supply current authority.
type UnityAssetReferenceOptions struct {
	Project               string
	FamilyID              string
	ReceiptPath           string
	ActivePath            string
	ExpectedEditorVersion string
	ExpectedEditorSHA256  string
	ExpectedBuildTarget   string
}

var ErrAssetReferencesRefused = assetrefs.ErrRefused

func (c *Client) assetReferenceOptions(ctx context.Context, service string, opts UnityAssetReferenceOptions) (assetrefs.Options, error) {
	root, _, found, err := c.ServiceMeta(ctx, service)
	if err != nil {
		return assetrefs.Options{}, fmt.Errorf("client.assetReferences: service metadata: %w", err)
	}
	if !found || root == "" {
		return assetrefs.Options{}, fmt.Errorf("%w: registered service root required", ErrAssetReferencesRefused)
	}
	return assetrefs.Options{Root: root, Service: service, Project: opts.Project, FamilyID: opts.FamilyID, ReceiptPath: opts.ReceiptPath, ActivePath: opts.ActivePath, ExpectedEditorVersion: opts.ExpectedEditorVersion, ExpectedEditorSHA256: opts.ExpectedEditorSHA256, ExpectedBuildTarget: opts.ExpectedBuildTarget}, nil
}

// PrepareUnityAssetReferences validates current source/receipt/native identity
// and returns a partial candidate DTO. It never persists or registers facts.
func (c *Client) PrepareUnityAssetReferences(ctx context.Context, service string, opts UnityAssetReferenceOptions) (AssetReferenceSnapshot, error) {
	local, err := c.assetReferenceOptions(ctx, service, opts)
	if err != nil {
		return AssetReferenceSnapshot{}, err
	}
	return assetrefs.Prepare(ctx, local)
}

// UnityAssetReferences validates current local proof again; a caller-supplied
// prepared snapshot is deliberately not accepted as current query authority.
func (c *Client) UnityAssetReferences(ctx context.Context, service string, opts UnityAssetReferenceOptions, query AssetReferenceQuery) (AssetReferenceResult, error) {
	local, err := c.assetReferenceOptions(ctx, service, opts)
	if err != nil {
		return AssetReferenceResult{}, err
	}
	return assetrefs.Query(ctx, local, query)
}
