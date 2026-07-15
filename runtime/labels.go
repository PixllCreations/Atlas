package runtime

// Atlas ownership and component labels.
const (
	LabelManagedBy      = "app.kubernetes.io/managed-by"
	LabelManagedByValue = "atlas"

	LabelProjectID   = "atlas.edwardscott.dev/project-id"
	LabelProjectName = "atlas.edwardscott.dev/project-name"
	LabelComponent   = "atlas.edwardscott.dev/component"
	LabelDepName     = "atlas.edwardscott.dev/dependency-name"
	LabelDepType     = "atlas.edwardscott.dev/dependency-type"

	ComponentApplication = "application"
	ComponentDependency  = "dependency"
)

// ProjectLabels returns base ownership labels for a project.
func ProjectLabels(projectID, projectName string) map[string]string {
	labels := map[string]string{
		LabelManagedBy: LabelManagedByValue,
	}
	if projectID != "" {
		labels[LabelProjectID] = projectID
	}
	if projectName != "" {
		labels[LabelProjectName] = projectName
	}
	return labels
}

// MergeLabels copies base then overlays extra keys.
func MergeLabels(base map[string]string, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
