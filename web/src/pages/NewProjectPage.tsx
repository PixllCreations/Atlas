import { useState, type SubmitEvent } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";

export function NewProjectPage() {
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  async function onSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      const app = await api.createApp(name.trim());
      navigate(`/projects/${app.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create");
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <div className="page-header">
        <div>
          <h1>New project</h1>
          <p>Names become Kubernetes resource names and ingress hosts.</p>
        </div>
      </div>

      <div className="panel">
        <form
          className="form"
          onSubmit={onSubmit}
        >
          <div className="field">
            <label htmlFor="name">Project name</label>
            <input
              id="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="portfolio"
              required
              pattern="[a-z0-9]([-a-z0-9]*[a-z0-9])?"
              title="Lowercase alphanumeric with optional hyphens"
              autoFocus
            />
          </div>
          {error && <p className="error">{error}</p>}
          <button
            className="btn btn-primary"
            type="submit"
            disabled={saving || !name.trim()}
          >
            {saving ? "Creating…" : "Create project"}
          </button>
        </form>
      </div>
    </>
  );
}
