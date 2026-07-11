package runtime

import "testing"

func TestIngressTLS(t *testing.T) {
	t.Run("empty secret omits TLS", func(t *testing.T) {
		got := ingressTLS(IngressOptions{Host: "app.homelab.local"})
		if got != nil {
			t.Fatalf("ingressTLS() = %+v, want nil", got)
		}
	})

	t.Run("secret enables TLS for host", func(t *testing.T) {
		got := ingressTLS(IngressOptions{
			Host:          "portfolio.homelab.local",
			TLSSecretName: "homelab-tls",
		})
		if len(got) != 1 {
			t.Fatalf("len(TLS) = %d, want 1", len(got))
		}
		if got[0].SecretName != "homelab-tls" {
			t.Fatalf("SecretName = %q, want homelab-tls", got[0].SecretName)
		}
		if len(got[0].Hosts) != 1 || got[0].Hosts[0] != "portfolio.homelab.local" {
			t.Fatalf("Hosts = %v, want [portfolio.homelab.local]", got[0].Hosts)
		}
	})
}

func TestDesiredIngressTLS(t *testing.T) {
	ing := desiredIngress(IngressOptions{
		Name:          "portfolio",
		Host:          "portfolio.homelab.local",
		TLSSecretName: "homelab-tls",
	})
	if len(ing.Spec.TLS) != 1 {
		t.Fatalf("len(Spec.TLS) = %d, want 1", len(ing.Spec.TLS))
	}
	if ing.Spec.TLS[0].SecretName != "homelab-tls" {
		t.Fatalf("SecretName = %q, want homelab-tls", ing.Spec.TLS[0].SecretName)
	}
	if ing.Spec.Rules[0].Host != "portfolio.homelab.local" {
		t.Fatalf("Host = %q, want portfolio.homelab.local", ing.Spec.Rules[0].Host)
	}
}
