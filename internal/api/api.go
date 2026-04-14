package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexfsong/jeeves/internal/config"
	"github.com/alexfsong/jeeves/internal/engine"
	"github.com/alexfsong/jeeves/internal/llm"
	"github.com/alexfsong/jeeves/internal/resolution"
	"github.com/alexfsong/jeeves/internal/store"
)

type API struct {
	store  *store.Store
	engine *engine.Engine
	router *llm.Router
	cfg    config.Config
}

func New(st *store.Store, eng *engine.Engine, router *llm.Router, cfg config.Config) *API {
	return &API{store: st, engine: eng, router: router, cfg: cfg}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()

	// Topics
	mux.HandleFunc("GET /api/topics", a.listTopics)
	mux.HandleFunc("POST /api/topics", a.createTopic)
	mux.HandleFunc("DELETE /api/topics/{slug}", a.deleteTopic)
	mux.HandleFunc("GET /api/topics/{slug}", a.getTopic)
	mux.HandleFunc("GET /api/topics/{slug}/knowledge", a.topicKnowledge)
	mux.HandleFunc("GET /api/topics/{slug}/branches", a.topicBranches)
	mux.HandleFunc("GET /api/topics/{slug}/ancestors", a.topicAncestors)
	mux.HandleFunc("POST /api/topics/{slug}/branch", a.branchTopic)

	// Topic links
	mux.HandleFunc("GET /api/topic-links", a.listTopicLinks)
	mux.HandleFunc("POST /api/topic-links", a.createTopicLink)

	// Knowledge
	mux.HandleFunc("GET /api/knowledge", a.listKnowledge)
	mux.HandleFunc("GET /api/knowledge/search", a.searchKnowledge)
	mux.HandleFunc("GET /api/knowledge/graph", a.knowledgeGraph)
	mux.HandleFunc("GET /api/knowledge/{id}", a.getKnowledge)

	// Research
	mux.HandleFunc("POST /api/research", a.research)

	// Trusted sources
	mux.HandleFunc("GET /api/trusted-sources", a.listTrustedSources)
	mux.HandleFunc("POST /api/trusted-sources", a.addTrustedSource)
	mux.HandleFunc("DELETE /api/trusted-sources/{id}", a.removeTrustedSource)

	// Stats & sessions
	mux.HandleFunc("GET /api/stats", a.getStats)
	mux.HandleFunc("GET /api/sessions", a.listSessions)

	// Config & status
	mux.HandleFunc("GET /api/status", a.getStatus)
	mux.HandleFunc("GET /api/ollama/models", a.listOllamaModels)
	mux.HandleFunc("PUT /api/config/model", a.setModel)

	// Follow-up suggestions
	mux.HandleFunc("POST /api/research/follow-ups", a.generateFollowUps)

	// Auto-topic from research
	mux.HandleFunc("POST /api/research/with-topic", a.researchWithAutoTopic)

	// Intelligence layer
	mux.HandleFunc("GET /api/knowledge/prior", a.priorKnowledge)
	mux.HandleFunc("GET /api/topics/{slug}/gaps", a.gapAnalysis)

	return corsMiddleware(mux)
}

// HandlerWithStatic returns a handler that serves API routes and falls back to static files.
func (a *API) HandlerWithStatic(staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	// Mount API routes
	apiHandler := a.Handler()
	mux.Handle("/api/", apiHandler)

	// Static files fallback
	mux.Handle("/", http.FileServerFS(staticFS))

	return mux
}

// --- Topics ---

func (a *API) listTopics(w http.ResponseWriter, r *http.Request) {
	topics, err := a.store.ListTopics()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type topicWithCount struct {
		store.Topic
		KnowledgeCount int `json:"knowledge_count"`
	}

	result := make([]topicWithCount, 0, len(topics))
	for _, t := range topics {
		count, _ := a.store.TopicKnowledgeCount(t.ID)
		result = append(result, topicWithCount{Topic: t, KnowledgeCount: count})
	}

	jsonOK(w, result)
}

func (a *API) createTopic(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	topic, err := a.store.CreateTopic(body.Name, body.Description)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, topic)
}

func (a *API) deleteTopic(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if err := a.store.DeleteTopic(slug); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

func (a *API) getTopic(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	topic, err := a.store.GetTopicBySlug(slug)
	if err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	count, _ := a.store.TopicKnowledgeCount(topic.ID)
	jsonOK(w, map[string]any{
		"topic":           topic,
		"knowledge_count": count,
	})
}

func (a *API) topicKnowledge(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	topic, err := a.store.GetTopicBySlug(slug)
	if err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	knowledge, err := a.store.ListKnowledgeByTopic(topic.ID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, emptySlice(knowledge))
}

func (a *API) topicBranches(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	topic, err := a.store.GetTopicBySlug(slug)
	if err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	branches, err := a.store.GetTopicBranches(topic.ID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, emptySlice(branches))
}

func (a *API) topicAncestors(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	topic, err := a.store.GetTopicBySlug(slug)
	if err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	ancestors, err := a.store.GetBranchAncestors(topic.ID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, emptySlice(ancestors))
}

func (a *API) branchTopic(w http.ResponseWriter, r *http.Request) {
	parentSlug := r.PathValue("slug")
	parent, err := a.store.GetTopicBySlug(parentSlug)
	if err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	child, err := a.store.CreateTopic(body.Name, body.Description)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := a.store.AddTopicLink(child.ID, parent.ID, "branched_from"); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, child)
}

// --- Topic Links ---

func (a *API) listTopicLinks(w http.ResponseWriter, r *http.Request) {
	links, err := a.store.ListTopicLinks()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, emptySlice(links))
}

func (a *API) createTopicLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FromSlug string `json:"from_slug"`
		ToSlug   string `json:"to_slug"`
		Kind     string `json:"kind"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	from, err := a.store.GetTopicBySlug(body.FromSlug)
	if err != nil {
		jsonError(w, fmt.Sprintf("from topic: %v", err), http.StatusNotFound)
		return
	}
	to, err := a.store.GetTopicBySlug(body.ToSlug)
	if err != nil {
		jsonError(w, fmt.Sprintf("to topic: %v", err), http.StatusNotFound)
		return
	}

	if err := a.store.AddTopicLink(from.ID, to.ID, body.Kind); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "linked"})
}

// --- Knowledge ---

func (a *API) listKnowledge(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	knowledge, err := a.store.ListAllKnowledge(limit)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, emptySlice(knowledge))
}

func (a *API) getKnowledge(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	k, err := a.store.GetKnowledge(id)
	if err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	jsonOK(w, k)
}

func (a *API) searchKnowledge(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		jsonError(w, "q parameter is required", http.StatusBadRequest)
		return
	}

	results, err := a.store.SearchKnowledge(q, 20)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, emptySlice(results))
}

func (a *API) knowledgeGraph(w http.ResponseWriter, r *http.Request) {
	var topicID *int64
	if slug := r.URL.Query().Get("topic"); slug != "" {
		topic, err := a.store.GetTopicBySlug(slug)
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		topicID = &topic.ID
	}

	graph, err := a.store.GetKnowledgeGraph(topicID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, graph)
}

// --- Research (SSE) ---

func (a *API) research(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query      string `json:"query"`
		Resolution string `json:"resolution"`
		TopicSlug  string `json:"topic_slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Query == "" {
		jsonError(w, "query is required", http.StatusBadRequest)
		return
	}

	resStr := body.Resolution
	if resStr == "" {
		resStr = a.cfg.Defaults.Resolution
	}
	res, err := resolution.Parse(resStr)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var topicID *int64
	if body.TopicSlug != "" {
		topic, err := a.store.GetTopicBySlug(body.TopicSlug)
		if err != nil {
			jsonError(w, fmt.Sprintf("topic: %v", err), http.StatusNotFound)
			return
		}
		topicID = &topic.ID
	}

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sendSSE := func(event, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	sendSSE("status", "Searching...")

	result, err := a.runResearchStream(r.Context(), body.Query, res, topicID, sendSSE)
	if err != nil {
		sendSSE("error", err.Error())
		return
	}

	// Log session
	if a.store != nil {
		a.store.LogSession(topicID, body.Query, res.String(), len(result.Results))
	}

	sendSSE("status", "Complete")

	resultJSON, _ := json.Marshal(result)
	sendSSE("result", string(resultJSON))
}

// runResearchStream runs the engine's research pipeline with a progress
// channel and relays each event as a `progress` SSE frame. Returns when
// the pipeline completes.
func (a *API) runResearchStream(
	ctx context.Context,
	query string,
	res resolution.Level,
	topicID *int64,
	sendSSE func(event, data string),
) (*engine.ResearchResult, error) {
	progress := make(chan engine.ProgressEvent, 8)

	type outcome struct {
		result *engine.ResearchResult
		err    error
	}
	done := make(chan outcome, 1)

	go func() {
		result, err := a.engine.ResearchStream(ctx, query, res, topicID, progress)
		close(progress)
		done <- outcome{result: result, err: err}
	}()

	for ev := range progress {
		if b, err := json.Marshal(ev); err == nil {
			sendSSE("progress", string(b))
		}
	}

	o := <-done
	return o.result, o.err
}

// --- Trusted Sources ---

func (a *API) listTrustedSources(w http.ResponseWriter, r *http.Request) {
	var topicID *int64
	if slug := r.URL.Query().Get("topic"); slug != "" {
		topic, err := a.store.GetTopicBySlug(slug)
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		topicID = &topic.ID
	}

	sources, err := a.store.ListTrustedSources(topicID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, emptySlice(sources))
}

func (a *API) addTrustedSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain     string  `json:"domain"`
		TrustLevel float64 `json:"trust_level"`
		TopicSlug  string  `json:"topic_slug"`
		Notes      string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	domain := strings.TrimPrefix(body.Domain, "www.")
	if domain == "" {
		jsonError(w, "domain is required", http.StatusBadRequest)
		return
	}
	if body.TrustLevel == 0 {
		body.TrustLevel = 1.5 // default boost
	}

	var topicID *int64
	if body.TopicSlug != "" {
		topic, err := a.store.GetTopicBySlug(body.TopicSlug)
		if err != nil {
			jsonError(w, fmt.Sprintf("topic: %v", err), http.StatusNotFound)
			return
		}
		topicID = &topic.ID
	}

	ts, err := a.store.AddTrustedSource(domain, body.TrustLevel, topicID, body.Notes)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, ts)
}

func (a *API) removeTrustedSource(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := a.store.RemoveTrustedSource(id); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]string{"status": "removed"})
}

// --- Stats & Sessions ---

func (a *API) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := a.store.GetStats()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, stats)
}

func (a *API) listSessions(w http.ResponseWriter, r *http.Request) {
	var topicID *int64
	if slug := r.URL.Query().Get("topic"); slug != "" {
		topic, err := a.store.GetTopicBySlug(slug)
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		topicID = &topic.ID
	}

	sessions, err := a.store.ListSessions(topicID, 50)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, emptySlice(sessions))
}

// --- Config & Status ---

func (a *API) getStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]any{
		"search_provider": a.cfg.Search.Provider,
		"brave_configured": a.cfg.Search.BraveAPIKey != "",
		"tavily_configured": a.cfg.Search.TavilyAPIKey != "",
		"cloud_provider":   a.cfg.LLM.CloudProvider,
		"cloud_model":      a.cfg.LLM.CloudModel,
		"cloud_configured": a.cfg.LLM.CloudAPIKey != "",
		"local_provider":   a.cfg.LLM.LocalProvider,
		"local_available":  a.router.LocalAvailable(),
		"cloud_available":  a.router.CloudAvailable(),
		"default_resolution": a.cfg.Defaults.Resolution,
		"verify_enabled":     a.cfg.Verify.Enabled,
	}

	if ollama := a.router.LocalOllama(); ollama != nil {
		status["local_model"] = ollama.Model()
		status["local_endpoint"] = ollama.Endpoint()
	}

	jsonOK(w, status)
}

func (a *API) listOllamaModels(w http.ResponseWriter, r *http.Request) {
	ollama := a.router.LocalOllama()
	if ollama == nil {
		jsonOK(w, []string{})
		return
	}

	models, err := ollama.ListModels()
	if err != nil {
		jsonOK(w, map[string]any{
			"models": []string{},
			"error":  err.Error(),
		})
		return
	}
	jsonOK(w, map[string]any{"models": models})
}

func (a *API) setModel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model == "" {
		jsonError(w, "model is required", http.StatusBadRequest)
		return
	}

	ollama := a.router.LocalOllama()
	if ollama == nil {
		jsonError(w, "ollama not available", http.StatusServiceUnavailable)
		return
	}

	ollama.SetModel(body.Model)
	jsonOK(w, map[string]string{"status": "ok", "model": body.Model})
}

// --- Follow-up suggestions ---

func (a *API) generateFollowUps(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query     string `json:"query"`
		Synthesis string `json:"synthesis"`
		TopicSlug string `json:"topic_slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	followUps, err := a.engine.GenerateFollowUps(r.Context(), body.Query, body.Synthesis)
	if err != nil {
		// Fallback: return empty rather than error
		jsonOK(w, map[string]any{"follow_ups": []string{}})
		return
	}

	jsonOK(w, map[string]any{"follow_ups": followUps})
}

// --- Research with auto-topic creation ---

func (a *API) researchWithAutoTopic(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query      string `json:"query"`
		Resolution string `json:"resolution"`
		TopicName  string `json:"topic_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Query == "" {
		jsonError(w, "query is required", http.StatusBadRequest)
		return
	}

	resStr := body.Resolution
	if resStr == "" {
		resStr = "detailed"
	}
	res, err := resolution.Parse(resStr)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Auto-create topic from the query if no name given
	topicName := body.TopicName
	if topicName == "" {
		topicName = body.Query
		if len(topicName) > 60 {
			topicName = topicName[:60]
		}
	}

	var topicID *int64
	topic, err := a.store.CreateTopic(topicName, "Auto-created from research")
	if err != nil {
		// Might already exist with this slug, try to find it
		existing, findErr := a.store.FindTopicByName(topicName)
		if findErr == nil && existing != nil {
			topicID = &existing.ID
		}
	} else {
		topicID = &topic.ID
	}

	// SSE stream
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sendSSE := func(event, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	sendSSE("status", "Researching...")

	result, err := a.runResearchStream(r.Context(), body.Query, res, topicID, sendSSE)
	if err != nil {
		sendSSE("error", err.Error())
		return
	}

	if a.store != nil {
		a.store.LogSession(topicID, body.Query, res.String(), len(result.Results))
	}

	// Include topic info in response
	type richResult struct {
		*engine.ResearchResult
		TopicID   *int64 `json:"topic_id,omitempty"`
		TopicName string `json:"topic_name,omitempty"`
		TopicSlug string `json:"topic_slug,omitempty"`
	}
	rr := richResult{ResearchResult: result, TopicID: topicID}
	if topic != nil {
		rr.TopicName = topic.Name
		rr.TopicSlug = topic.Slug
	}

	sendSSE("status", "Complete")
	resultJSON, _ := json.Marshal(rr)
	sendSSE("result", string(resultJSON))
}

// --- Intelligence layer ---

func (a *API) priorKnowledge(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		jsonError(w, "q parameter is required", http.StatusBadRequest)
		return
	}

	prior, err := a.engine.PriorKnowledge(r.Context(), q)
	if err != nil {
		jsonOK(w, map[string]any{"entries": []any{}, "count": 0})
		return
	}

	jsonOK(w, map[string]any{
		"entries": emptySlice(prior),
		"count":   len(prior),
	})
}

func (a *API) gapAnalysis(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	topic, err := a.store.GetTopicBySlug(slug)
	if err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	analysis, err := a.engine.GapAnalysis(r.Context(), topic.ID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]string{"analysis": analysis})
}

// --- Helpers ---

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func emptySlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
