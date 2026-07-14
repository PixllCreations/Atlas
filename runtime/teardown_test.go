package runtime

import (
	"context"
	"testing"
)

func TestDeleteHelpers_RequireName(t *testing.T) {
	c := &Client{}
	ctx := context.Background()

	if err := c.DeleteDeployment(ctx, "default", ""); err == nil {
		t.Fatal("DeleteDeployment empty name: want error")
	}
	if err := c.DeleteService(ctx, "default", ""); err == nil {
		t.Fatal("DeleteService empty name: want error")
	}
	if err := c.DeleteIngress(ctx, "default", ""); err == nil {
		t.Fatal("DeleteIngress empty name: want error")
	}
}
