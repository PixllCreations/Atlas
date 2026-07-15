package runtime

import "testing"

func TestProjectLabels(t *testing.T) {
	labels := ProjectLabels("id-1", "demo")
	if labels[LabelManagedBy] != LabelManagedByValue {
		t.Fatalf("managed-by = %q", labels[LabelManagedBy])
	}
	if labels[LabelProjectID] != "id-1" {
		t.Fatalf("project-id = %q", labels[LabelProjectID])
	}
	if labels[LabelProjectName] != "demo" {
		t.Fatalf("project-name = %q", labels[LabelProjectName])
	}
}
