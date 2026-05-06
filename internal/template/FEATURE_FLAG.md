# Feature Flags

This file documents the feature flag system used by the template engine.

## Overview

Feature flags control template rendering behavior at runtime. They allow
conditional inclusion of template sections based on cluster configuration
and deployment state.

## Usage

Feature flags are evaluated during template rendering through the
`{{ if .FeatureEnabled "flag-name" }}` template function.

## Registered Flags

Flags are registered through the template engine's feature registry.
See `engine.go` for the registration API.
