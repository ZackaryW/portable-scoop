package resolver

import "portscoop/internal/localvol"

type Volume = localvol.Volume

// VolumeSource returns only supported local candidate volumes. Implementations
// must honor cancellation and bound probing of unavailable storage.
type VolumeSource = localvol.Source
