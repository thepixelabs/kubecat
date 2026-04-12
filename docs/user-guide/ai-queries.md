# AI Queries

Kubecat's AI integration lets you ask questions about your cluster in plain English and get contextual answers grounded in live resource data.

---

## Asking a Query

1. Click **AI** in the sidebar.
2. Type your question in the input box at the bottom.
3. Kubecat builds a context from the current cluster state and sends it to your configured AI provider.
4. The response streams in with a typing animation.

**Example queries:**

- "Why is my nginx pod in CrashLoopBackOff?"
- "What services are exposed externally?"
- "Do any pods lack resource limits?"
- "Summarize the recent events in the default namespace"
- "What changed in the last hour?"

---

## What Gets Sent to the AI Provider

Kubecat sends only the data needed to answer your question:

- The resource(s) you are asking about (spec, status, recent events)
- A system prompt describing the Kubernetes context

**What is never sent:**
- Kubernetes Secret values
- API keys or tokens from pod environment variables
- Full cluster state (only the relevant subset)

All data is sanitized by `SanitizeForCloud` before leaving your machine. See `PRIVACY.md` for details.

---

## Configuring Providers

Open **Settings → AI** to configure your provider.

### Ollama (local, recommended)

Ollama runs models locally — zero data leaves your machine.

1. Install Ollama: https://ollama.ai
2. Pull a model: `ollama pull llama3.2`
3. In Kubecat Settings → AI, set provider to **Ollama** and endpoint to `http://localhost:11434`.
4. Select the model from the dropdown.

### OpenAI

Requires an OpenAI API key. Models: `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`.

### Anthropic

Requires an Anthropic API key. Models: `claude-3-5-sonnet-20241022`, `claude-3-haiku-20240307`.

### Google Gemini

Requires a Google AI API key. Models: `gemini-2.0-flash`, `gemini-1.5-pro`.

---

## Conversation History

Each AI session maintains a conversation history. Previous messages and responses are shown above the input box. The AI can refer to earlier context in follow-up questions.

Click **New Conversation** to start fresh.

---

## Safety

AI-suggested commands are displayed but **never automatically executed**. Any `kubectl` command appearing in an AI response is shown as a code block with a **Copy** button. You decide what to run and where.

Autopilot mode (automatic command execution) is disabled by default. It remains disabled until the command safety classifier is hardened (see the security hardening roadmap).

---

## Analyzing a Specific Resource

From the Resource Explorer, open any resource and click **Ask AI** in the detail panel. This opens an AI query pre-loaded with the resource's context, so you can immediately ask "What's wrong with this pod?" without having to describe it.
