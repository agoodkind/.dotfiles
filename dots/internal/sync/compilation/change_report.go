package compilation

// WriteOutcome is the result of attempting to write a managed corpus file.
type WriteOutcome int

const (
	// WriteCreated means the destination did not exist and was written.
	WriteCreated WriteOutcome = iota
	// WriteUpdated means an existing destination was replaced with new content.
	WriteUpdated
	// WriteUnchanged means the destination already matched the new content.
	WriteUnchanged
	// WriteSkippedUnmanaged means an existing non-generated file was left alone.
	WriteSkippedUnmanaged
)

// ChangeReport accumulates file write and removal outcomes for one render call.
type ChangeReport struct {
	Created          []string
	Updated          []string
	Unchanged        []string
	SkippedUnmanaged []string
	Removed          []string
}

func emptyChangeReport() ChangeReport {
	return ChangeReport{
		Created:          nil,
		Updated:          nil,
		Unchanged:        nil,
		SkippedUnmanaged: nil,
		Removed:          nil,
	}
}

// Merge appends every path from other into the receiver.
//
// Sync calls Merge after each provider render so one summary covers every
// destination written during the run.
//
// Example:
//
//	total := emptyChangeReport()
//	total.Merge(rulesReport)
//	total.Merge(skillsReport)
func (r *ChangeReport) Merge(other ChangeReport) {
	if r == nil {
		return
	}

	r.Created = append(r.Created, other.Created...)
	r.Updated = append(r.Updated, other.Updated...)
	r.Unchanged = append(r.Unchanged, other.Unchanged...)
	r.SkippedUnmanaged = append(r.SkippedUnmanaged, other.SkippedUnmanaged...)
	r.Removed = append(r.Removed, other.Removed...)
}

// WrittenCount returns the number of created plus updated paths.
//
// Sync uses WrittenCount in the corpus summary line so operators see how many
// managed files changed during the run.
//
// Example:
//
//	common.InfoContextf(ctx, logger, "written: %d", report.WrittenCount())
func (r *ChangeReport) WrittenCount() int {
	if r == nil {
		return 0
	}

	return len(r.Created) + len(r.Updated)
}

func (r *ChangeReport) recordWrite(path string, outcome WriteOutcome) {
	if r == nil {
		return
	}

	switch outcome {
	case WriteCreated:
		r.Created = append(r.Created, path)
	case WriteUpdated:
		r.Updated = append(r.Updated, path)
	case WriteUnchanged:
		r.Unchanged = append(r.Unchanged, path)
	case WriteSkippedUnmanaged:
		r.SkippedUnmanaged = append(r.SkippedUnmanaged, path)
	}
}

func (r *ChangeReport) recordRemoved(paths ...string) {
	if r == nil {
		return
	}

	r.Removed = append(r.Removed, paths...)
}
