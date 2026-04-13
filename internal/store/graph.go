package store

type KnowledgeNode struct {
	ID      int64         `json:"id"`
	Title   string        `json:"title"`
	Type    KnowledgeType `json:"type"`
	TopicID *int64        `json:"topic_id,omitempty"`
	Score   float64       `json:"score"`
}

type KnowledgeEdge struct {
	FromID int64  `json:"from_id"`
	ToID   int64  `json:"to_id"`
	Kind   string `json:"kind"`
}

type Graph struct {
	Nodes []KnowledgeNode `json:"nodes"`
	Edges []KnowledgeEdge `json:"edges"`
}

func (g *Graph) EnsureNonNil() {
	if g.Nodes == nil {
		g.Nodes = []KnowledgeNode{}
	}
	if g.Edges == nil {
		g.Edges = []KnowledgeEdge{}
	}
}

// GetKnowledgeGraph returns nodes and edges for visualization.
// If topicID is non-nil, filters to that topic's knowledge.
func (s *Store) GetKnowledgeGraph(topicID *int64) (*Graph, error) {
	g := &Graph{}

	// Fetch nodes
	var nodeQuery string
	var nodeArgs []any
	if topicID != nil {
		nodeQuery = "SELECT id, title, type, topic_id, score FROM knowledge WHERE topic_id = ?"
		nodeArgs = []any{*topicID}
	} else {
		nodeQuery = "SELECT id, title, type, topic_id, score FROM knowledge"
	}

	rows, err := s.db.Query(nodeQuery, nodeArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodeIDs := make(map[int64]bool)
	for rows.Next() {
		var n KnowledgeNode
		if err := rows.Scan(&n.ID, &n.Title, &n.Type, &n.TopicID, &n.Score); err != nil {
			return nil, err
		}
		g.Nodes = append(g.Nodes, n)
		nodeIDs[n.ID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fetch edges (only those connecting nodes in our set)
	edgeRows, err := s.db.Query(
		"SELECT from_id, to_id, kind FROM knowledge_links",
	)
	if err != nil {
		return nil, err
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		var e KnowledgeEdge
		if err := edgeRows.Scan(&e.FromID, &e.ToID, &e.Kind); err != nil {
			return nil, err
		}
		if nodeIDs[e.FromID] && nodeIDs[e.ToID] {
			g.Edges = append(g.Edges, e)
		}
	}

	g.EnsureNonNil()
	return g, edgeRows.Err()
}

type Stats struct {
	TotalEntries   int            `json:"total_entries"`
	ByType         map[string]int `json:"by_type"`
	ByTopic        map[string]int `json:"by_topic"`
	TotalTopics    int            `json:"total_topics"`
	TotalSessions  int            `json:"total_sessions"`
	RecentSessions []Session      `json:"recent_sessions"`
}

// GetStats returns aggregate statistics for the dashboard overview.
func (s *Store) GetStats() (*Stats, error) {
	st := &Stats{
		ByType:  make(map[string]int),
		ByTopic: make(map[string]int),
	}

	// Total entries
	s.db.QueryRow("SELECT COUNT(*) FROM knowledge").Scan(&st.TotalEntries)

	// By type
	rows, err := s.db.Query("SELECT type, COUNT(*) FROM knowledge GROUP BY type")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t string
		var c int
		rows.Scan(&t, &c)
		st.ByType[t] = c
	}
	rows.Close()

	// By topic
	rows, err = s.db.Query(
		`SELECT t.name, COUNT(k.id) FROM topics t
		 LEFT JOIN knowledge k ON k.topic_id = t.id
		 GROUP BY t.id ORDER BY COUNT(k.id) DESC`,
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var name string
		var c int
		rows.Scan(&name, &c)
		st.ByTopic[name] = c
	}
	rows.Close()

	// Totals
	s.db.QueryRow("SELECT COUNT(*) FROM topics").Scan(&st.TotalTopics)
	s.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&st.TotalSessions)

	// Recent sessions
	st.RecentSessions, _ = s.ListSessions(nil, 20)

	return st, nil
}
