package codexhistory

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"vibe-tree/backend/internal/id"
	"vibe-tree/backend/internal/store"

	_ "modernc.org/sqlite"
)

const (
	defaultImportedProvider = "openai"
	defaultImportedModel    = "gpt-5-codex"
	defaultCLIToolID        = "codex"
)

var (
	reWorkerTitle = regexp.MustCompile(`(?m)^你是并行 worker\s+([^\s。]+)`)
	reTaskTitle   = regexp.MustCompile(`(?m)^\s*-\s*task_title:\s*(.+?)\s*$`)
	reUserLine    = regexp.MustCompile(`(?m)^\s*USER:\s*(.+?)\s*$`)
	reExitCode    = regexp.MustCompile(`Process exited with code\s+(-?\d+)`)
)

type Options struct {
	SourceRoot  string
	StateDBPath string
}

type Service struct {
	store       *store.Store
	sourceRoot  string
	stateDBPath string
}

type ThreadSummary struct {
	ThreadID        string `json:"thread_id"`
	DisplayTitle    string `json:"display_title"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	WorkspacePath   string `json:"workspace_path"`
	Source          string `json:"source"`
	ModelProvider   string `json:"model_provider"`
	Archived        bool   `json:"archived"`
	AlreadyImported bool   `json:"already_imported"`
}

type ImportResult struct {
	ThreadID        string `json:"thread_id"`
	DisplayTitle    string `json:"display_title"`
	SessionID       string `json:"session_id,omitempty"`
	Imported        bool   `json:"imported"`
	AlreadyImported bool   `json:"already_imported"`
}

type ImportResponse struct {
	Results []ImportResult `json:"results"`
}

type threadRow struct {
	ID               string
	RolloutPath      string
	CreatedAt        int64
	UpdatedAt        int64
	Source           string
	ModelProvider    string
	Cwd              string
	Title            string
	Archived         bool
	FirstUserMessage string
}

type parsedRollout struct {
	Messages             []store.ImportedChatMessage
	Turns                []store.ImportedChatTurn
	LastActivityAt       int64
	FirstRealUserMessage string
}

type parsedLine struct {
	Timestamp string         `json:"timestamp"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
}

type parsedTurn struct {
	number            int64
	userMessage       store.ImportedChatMessage
	items             []store.ImportedChatTurnItem
	nextSeq           int
	itemIndexByEntry  map[string]int
	assistantText     string
	assistantAt       int64
	turnStartedAt     int64
	lastUpdatedAt     int64
	errorText         string
	lastAssistantText string
}

func NewService(st *store.Store, opts Options) *Service {
	return &Service{
		store:       st,
		sourceRoot:  strings.TrimSpace(opts.SourceRoot),
		stateDBPath: strings.TrimSpace(opts.StateDBPath),
	}
}

func (s *Service) ListThreads(ctx context.Context, limit int) ([]ThreadSummary, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	dbPaths, err := s.resolveStateDBPaths()
	if err != nil {
		return nil, err
	}

	imported, err := s.listImportedThreadIDs(ctx)
	if err != nil {
		return nil, err
	}

	merged := make(map[string]ThreadSummary, limit)
	var fallbackErr error
	for _, dbPath := range dbPaths {
		threads, err := s.listThreadsFromStateDB(ctx, dbPath, limit, imported)
		if err != nil {
			if isRecoverableStateDBError(err) {
				fallbackErr = err
				continue
			}
			return nil, err
		}
		for _, thread := range threads {
			existing, ok := merged[thread.ThreadID]
			if !ok || thread.UpdatedAt > existing.UpdatedAt {
				merged[thread.ThreadID] = thread
			}
		}
	}
	if len(merged) == 0 {
		if fallbackErr != nil {
			return nil, fallbackErr
		}
		return nil, fmt.Errorf("no readable codex state db found")
	}
	out := make([]ThreadSummary, 0, len(merged))
	for _, thread := range merged {
		out = append(out, thread)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].ThreadID < out[j].ThreadID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Service) ImportThreads(ctx context.Context, threadIDs []string) (ImportResponse, error) {
	ids := normalizeIDList(threadIDs)
	if len(ids) == 0 {
		return ImportResponse{}, fmt.Errorf("%w: thread_ids is required", store.ErrValidation)
	}
	dbPaths, err := s.resolveStateDBPaths()
	if err != nil {
		return ImportResponse{}, err
	}

	results := make([]ImportResult, 0, len(ids))
	for _, threadID := range ids {
		thread, err := s.findThreadByID(ctx, dbPaths, threadID)
		if err != nil {
			return ImportResponse{}, err
		}
		displayTitle := deriveDisplayTitle(thread.Title, thread.FirstUserMessage, thread.RolloutPath, thread.ID)
		parsed, err := parseRolloutFile(thread.RolloutPath)
		if err != nil {
			return ImportResponse{}, err
		}
		sess, created, err := s.store.ImportChatSession(ctx, store.ImportChatSessionParams{
			Title:         displayTitle,
			ExpertID:      defaultCLIToolID,
			CLIToolID:     stringPtr(defaultCLIToolID),
			CLISessionID:  stringPtr(thread.ID),
			Provider:      importedProviderFrom(thread.ModelProvider),
			Model:         defaultImportedModel,
			WorkspacePath: defaultWorkspace(thread.Cwd),
			Status:        "active",
			CreatedAt:     chooseImportedTime(thread.CreatedAt, firstCreatedAt(parsed.Messages, parsed.Turns)),
			UpdatedAt:     chooseImportedTime(thread.UpdatedAt, parsed.LastActivityAt),
			LastTurn:      maxImportedTurn(parsed.Messages, parsed.Turns),
			Messages:      parsed.Messages,
			Turns:         parsed.Turns,
		})
		if err != nil {
			return ImportResponse{}, err
		}
		results = append(results, ImportResult{
			ThreadID:        thread.ID,
			DisplayTitle:    displayTitle,
			SessionID:       sess.ID,
			Imported:        created,
			AlreadyImported: !created,
		})
	}

	return ImportResponse{Results: results}, nil
}

func (s *Service) resolveStateDBPaths() ([]string, error) {
	if s == nil {
		return nil, fmt.Errorf("codex history service not initialized")
	}
	if s.stateDBPath != "" {
		return []string{s.stateDBPath}, nil
	}
	root := s.sourceRoot
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		root = filepath.Join(home, ".codex")
	}
	candidates, err := filepath.Glob(filepath.Join(root, "state_*.sqlite"))
	if err != nil {
		return nil, fmt.Errorf("glob codex state db: %w", err)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("codex state db not found under %s", root)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		leftInfo, leftErr := os.Stat(candidates[i])
		rightInfo, rightErr := os.Stat(candidates[j])
		switch {
		case leftErr == nil && rightErr == nil:
			if !leftInfo.ModTime().Equal(rightInfo.ModTime()) {
				return leftInfo.ModTime().After(rightInfo.ModTime())
			}
		case leftErr == nil:
			return true
		case rightErr == nil:
			return false
		}
		return candidates[i] > candidates[j]
	})
	return candidates, nil
}

func (s *Service) listThreadsFromStateDB(ctx context.Context, dbPath string, limit int, imported map[string]bool) ([]ThreadSummary, error) {
	db, err := openReadOnlySQLite(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open codex state db %s: %w", filepath.Base(dbPath), err)
	}
	defer db.Close()

	rows, err := db.QueryContext(
		ctx,
		`SELECT id, rollout_path, created_at, updated_at, source, model_provider, cwd, title, archived, first_user_message
		   FROM threads
		  ORDER BY updated_at DESC
		  LIMIT ?;`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query codex threads from %s: %w", filepath.Base(dbPath), err)
	}
	defer rows.Close()

	out := make([]ThreadSummary, 0, limit)
	for rows.Next() {
		thread, err := scanThreadRow(rows)
		if err != nil {
			if len(out) > 0 && isRecoverableStateDBError(err) {
				return out, nil
			}
			return nil, fmt.Errorf("scan codex thread from %s: %w", filepath.Base(dbPath), err)
		}
		out = append(out, ThreadSummary{
			ThreadID:        thread.ID,
			DisplayTitle:    deriveDisplayTitle(thread.Title, thread.FirstUserMessage, thread.RolloutPath, thread.ID),
			CreatedAt:       normalizeImportedTime(thread.CreatedAt),
			UpdatedAt:       normalizeImportedTime(thread.UpdatedAt),
			WorkspacePath:   strings.TrimSpace(thread.Cwd),
			Source:          strings.TrimSpace(thread.Source),
			ModelProvider:   strings.TrimSpace(thread.ModelProvider),
			Archived:        thread.Archived,
			AlreadyImported: imported[thread.ID],
		})
	}
	if err := rows.Err(); err != nil {
		if len(out) > 0 && isRecoverableStateDBError(err) {
			return out, nil
		}
		return nil, fmt.Errorf("iterate codex threads from %s: %w", filepath.Base(dbPath), err)
	}
	return out, nil
}

func (s *Service) findThreadByID(ctx context.Context, dbPaths []string, threadID string) (threadRow, error) {
	var fallbackErr error
	for _, dbPath := range dbPaths {
		db, err := openReadOnlySQLite(dbPath)
		if err != nil {
			if isRecoverableStateDBError(err) {
				fallbackErr = fmt.Errorf("open codex state db %s: %w", filepath.Base(dbPath), err)
				continue
			}
			return threadRow{}, fmt.Errorf("open codex state db %s: %w", filepath.Base(dbPath), err)
		}
		thread, err := getThreadByID(ctx, db, threadID)
		_ = db.Close()
		if err == nil {
			return thread, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if isRecoverableStateDBError(err) {
			fallbackErr = fmt.Errorf("read codex thread %s from %s: %w", threadID, filepath.Base(dbPath), err)
			continue
		}
		return threadRow{}, fmt.Errorf("read codex thread %s from %s: %w", threadID, filepath.Base(dbPath), err)
	}
	if fallbackErr != nil {
		return threadRow{}, fallbackErr
	}
	return threadRow{}, os.ErrNotExist
}

func (s *Service) listImportedThreadIDs(ctx context.Context) (map[string]bool, error) {
	out := map[string]bool{}
	if s == nil || s.store == nil || s.store.DB() == nil {
		return out, nil
	}
	rows, err := s.store.DB().QueryContext(
		ctx,
		`SELECT cli_session_id
		   FROM chat_sessions
		  WHERE cli_tool_id = ?
		    AND cli_session_id IS NOT NULL
		    AND TRIM(cli_session_id) != '';`,
		defaultCLIToolID,
	)
	if err != nil {
		return nil, fmt.Errorf("query imported codex sessions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cliSessionID string
		if err := rows.Scan(&cliSessionID); err != nil {
			return nil, fmt.Errorf("scan imported cli_session_id: %w", err)
		}
		cliSessionID = strings.TrimSpace(cliSessionID)
		if cliSessionID != "" {
			out[cliSessionID] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imported codex sessions: %w", err)
	}
	return out, nil
}

func openReadOnlySQLite(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&immutable=1", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

func isRecoverableStateDBError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "database disk image is malformed") ||
		strings.Contains(text, "malformed") ||
		strings.Contains(text, "file is not a database") ||
		strings.Contains(text, "database corruption")
}

func scanThreadRow(scanner interface{ Scan(dest ...any) error }) (threadRow, error) {
	var thread threadRow
	var archived int
	if err := scanner.Scan(
		&thread.ID,
		&thread.RolloutPath,
		&thread.CreatedAt,
		&thread.UpdatedAt,
		&thread.Source,
		&thread.ModelProvider,
		&thread.Cwd,
		&thread.Title,
		&archived,
		&thread.FirstUserMessage,
	); err != nil {
		return threadRow{}, fmt.Errorf("scan codex thread: %w", err)
	}
	thread.Archived = archived != 0
	return thread, nil
}

func getThreadByID(ctx context.Context, db *sql.DB, threadID string) (threadRow, error) {
	row := db.QueryRowContext(
		ctx,
		`SELECT id, rollout_path, created_at, updated_at, source, model_provider, cwd, title, archived, first_user_message
		   FROM threads
		  WHERE id = ?
		  LIMIT 1;`,
		threadID,
	)
	thread, err := scanThreadRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "no rows") {
			return threadRow{}, os.ErrNotExist
		}
		return threadRow{}, err
	}
	return thread, nil
}

func deriveDisplayTitle(rawTitle, firstUserMessage, rolloutPath, threadID string) string {
	if title := parseReadableTitle(rawTitle); title != "" {
		return title
	}
	if title := compactTitle(firstUserMessage); title != "" {
		return title
	}
	if title := firstUserTitleFromRollout(rolloutPath); title != "" {
		return title
	}
	short := strings.TrimSpace(threadID)
	if utf8.RuneCountInString(short) > 12 {
		short = string([]rune(short)[:12])
	}
	return "Codex " + short
}

func parseReadableTitle(raw string) string {
	title := strings.TrimSpace(raw)
	if title == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(title), "codex ") && strings.Count(title, "-") >= 3 {
		return ""
	}
	if strings.HasPrefix(title, "Recent conversation:") {
		if idx := strings.Index(title, "Current user input:"); idx >= 0 {
			return compactTitle(title[idx+len("Current user input:"):])
		}
		if match := reUserLine.FindStringSubmatch(title); len(match) > 1 {
			return compactTitle(match[1])
		}
	}
	if match := reTaskTitle.FindStringSubmatch(title); len(match) > 1 {
		return compactTitle(match[1])
	}
	if match := reWorkerTitle.FindStringSubmatch(title); len(match) > 1 {
		worker := strings.TrimSpace(match[1])
		if task := taskTitleFromPrompt(title); task != "" {
			return compactTitle(worker + " · " + task)
		}
		return compactTitle("并行 worker " + worker)
	}
	return compactTitle(title)
}

func taskTitleFromPrompt(text string) string {
	if match := reTaskTitle.FindStringSubmatch(text); len(match) > 1 {
		return compactTitle(match[1])
	}
	return ""
}

func compactTitle(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# AGENTS.md") || strings.HasPrefix(line, "<INSTRUCTIONS>") {
			continue
		}
		text = line
		break
	}
	text = strings.Join(strings.Fields(text), " ")
	text = strings.Trim(text, "\"'`")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) > 72 {
		return strings.TrimSpace(string(runes[:72])) + "…"
	}
	return text
}

func firstUserTitleFromRollout(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for scanner.Scan() {
		var line parsedLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			return ""
		}
		if line.Type != "event_msg" || payloadType(line.Payload["type"]) != "user_message" {
			continue
		}
		return compactTitle(extractEventMessage(line.Payload))
	}
	return ""
}

func parseRolloutFile(path string) (parsedRollout, error) {
	file, err := os.Open(path)
	if err != nil {
		return parsedRollout{}, fmt.Errorf("open rollout file: %w", err)
	}
	defer file.Close()

	out := parsedRollout{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var (
		currentTurn          *parsedTurn
		turnNumber           int64
		pendingTaskStartedAt int64
	)
	for scanner.Scan() {
		var line parsedLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			return parsedRollout{}, fmt.Errorf("decode rollout jsonl: %w", err)
		}
		ts := parseEventTime(line.Timestamp)
		if ts > out.LastActivityAt {
			out.LastActivityAt = ts
		}
		switch line.Type {
		case "event_msg":
			switch payloadType(line.Payload["type"]) {
			case "task_started":
				pendingTaskStartedAt = ts
				if currentTurn != nil && (currentTurn.turnStartedAt == 0 || ts < currentTurn.turnStartedAt) {
					currentTurn.turnStartedAt = ts
				}
			case "user_message":
				if currentTurn != nil {
					finishTurn(&out, currentTurn, "本轮未显式结束。", true)
				}
				turnNumber++
				text := extractEventMessage(line.Payload)
				if out.FirstRealUserMessage == "" {
					out.FirstRealUserMessage = text
				}
				userCreatedAt := ts
				if pendingTaskStartedAt > 0 && (userCreatedAt == 0 || pendingTaskStartedAt < userCreatedAt) {
					userCreatedAt = pendingTaskStartedAt
				}
				currentTurn = &parsedTurn{
					number: turnNumber,
					userMessage: store.ImportedChatMessage{
						ID:          id.New("cm_"),
						Turn:        turnNumber,
						Role:        "user",
						ContentText: text,
						CreatedAt:   userCreatedAt,
					},
					items:            []store.ImportedChatTurnItem{},
					nextSeq:          1,
					itemIndexByEntry: map[string]int{},
					turnStartedAt:    userCreatedAt,
					lastUpdatedAt:    userCreatedAt,
				}
				pendingTaskStartedAt = 0
			case "agent_message":
				if currentTurn == nil {
					continue
				}
				currentTurn.addEntry("progress:"+strconv.Itoa(currentTurn.nextSeq), "progress", "done", extractEventMessage(line.Payload), nil, ts)
			case "task_complete":
				if currentTurn == nil {
					continue
				}
				finalText := compactAssistantMessage(line.Payload)
				if finalText == "" {
					finalText = currentTurn.lastAssistantText
				}
				currentTurn.assistantText = strings.TrimSpace(finalText)
				currentTurn.assistantAt = ts
				finishTurn(&out, currentTurn, "", false)
				currentTurn = nil
				pendingTaskStartedAt = 0
			case "turn_aborted":
				if currentTurn == nil {
					continue
				}
				reason := strings.TrimSpace(stringField(line.Payload, "reason"))
				message := "本轮已中止。"
				if reason != "" {
					message = "本轮已中止：" + reason
				}
				currentTurn.addEntry("error:turn_aborted", "error", "failed", message, nil, ts)
				currentTurn.errorText = message
				finishTurn(&out, currentTurn, message, true)
				currentTurn = nil
				pendingTaskStartedAt = 0
			}
		case "response_item":
			if currentTurn == nil {
				continue
			}
			switch payloadType(line.Payload["type"]) {
			case "message":
				if strings.TrimSpace(stringField(line.Payload, "role")) == "assistant" {
					if text := extractMessageContent(line.Payload); text != "" {
						currentTurn.lastAssistantText = text
					}
				}
			case "reasoning", "agent_reasoning":
				text := extractReasoningText(line.Payload)
				if text != "" {
					currentTurn.appendThinking(text, ts)
				}
			case "function_call":
				callID := strings.TrimSpace(stringField(line.Payload, "call_id"))
				if callID == "" {
					callID = fmt.Sprintf("tool-%d", currentTurn.nextSeq)
				}
				currentTurn.addOrUpdateTool(callID, toolContentFromCall(line.Payload), "created", nil, ts)
			case "function_call_output":
				callID := strings.TrimSpace(stringField(line.Payload, "call_id"))
				meta := parseFunctionCallOutput(line.Payload)
				currentTurn.addOrUpdateTool(callID, "", toolStatusFromMeta(meta), meta, ts)
			case "custom_tool_call":
				callID := strings.TrimSpace(stringField(line.Payload, "call_id"))
				if callID == "" {
					callID = fmt.Sprintf("tool-%d", currentTurn.nextSeq)
				}
				currentTurn.addOrUpdateTool(callID, toolContentFromCustomCall(line.Payload), "created", nil, ts)
			case "custom_tool_call_output":
				callID := strings.TrimSpace(stringField(line.Payload, "call_id"))
				meta := parseCustomToolCallOutput(line.Payload)
				currentTurn.addOrUpdateTool(callID, "", toolStatusFromMeta(meta), meta, ts)
			case "web_search_call":
				entryID := fmt.Sprintf("tool:web_search:%d", currentTurn.nextSeq)
				currentTurn.addEntry(entryID, "tool", "success", toolContentFromWebSearch(line.Payload), nil, ts)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return parsedRollout{}, fmt.Errorf("scan rollout jsonl: %w", err)
	}
	if currentTurn != nil {
		finishTurn(&out, currentTurn, "本轮未完整结束。", true)
	}
	return out, nil
}

func finishTurn(out *parsedRollout, turn *parsedTurn, fallbackAssistant string, includeError bool) {
	if out == nil || turn == nil {
		return
	}
	userMsg := turn.userMessage
	if strings.TrimSpace(userMsg.ID) == "" {
		return
	}
	if userMsg.CreatedAt <= 0 {
		userMsg.CreatedAt = turn.turnStartedAt
	}
	out.Messages = append(out.Messages, userMsg)

	assistantText := strings.TrimSpace(turn.assistantText)
	if assistantText == "" {
		assistantText = strings.TrimSpace(fallbackAssistant)
	}
	if assistantText == "" && len(turn.items) > 0 {
		assistantText = "本轮未生成最终回复。"
	}
	assistantAt := turn.assistantAt
	if assistantAt <= 0 {
		assistantAt = turn.lastUpdatedAt
	}

	var assistantMessageID *string
	if assistantText != "" {
		if _, ok := turn.itemIndexByEntry["answer"]; !ok {
			turn.addEntry("answer", "answer", "done", assistantText, nil, assistantAt)
		}
		msgID := id.New("cm_")
		out.Messages = append(out.Messages, store.ImportedChatMessage{
			ID:          msgID,
			Turn:        turn.number,
			Role:        "assistant",
			ContentText: assistantText,
			CreatedAt:   assistantAt,
		})
		assistantMessageID = &msgID
	}

	turnCreatedAt := turn.turnStartedAt
	if turnCreatedAt <= 0 {
		turnCreatedAt = userMsg.CreatedAt
	}
	turnUpdatedAt := turn.lastUpdatedAt
	if assistantAt > turnUpdatedAt {
		turnUpdatedAt = assistantAt
	}
	if turnUpdatedAt <= 0 {
		turnUpdatedAt = turnCreatedAt
	}
	completedAt := turnUpdatedAt
	out.Turns = append(out.Turns, store.ImportedChatTurn{
		ID:                 id.New("ct_"),
		UserMessageID:      userMsg.ID,
		AssistantMessageID: assistantMessageID,
		Turn:               turn.number,
		Status:             "completed",
		CreatedAt:          turnCreatedAt,
		UpdatedAt:          turnUpdatedAt,
		CompletedAt:        &completedAt,
		Items:              turn.items,
	})
}

func (t *parsedTurn) appendThinking(text string, ts int64) {
	if t == nil || strings.TrimSpace(text) == "" {
		return
	}
	if idx, ok := t.itemIndexByEntry["thinking:1"]; ok {
		entry := t.items[idx]
		if entry.ContentText != "" {
			entry.ContentText += "\n\n"
		}
		entry.ContentText += text
		entry.UpdatedAt = chooseImportedTime(entry.UpdatedAt, ts)
		t.items[idx] = entry
		t.lastUpdatedAt = chooseImportedTime(t.lastUpdatedAt, ts)
		return
	}
	t.addEntry("thinking:1", "thinking", "done", text, nil, ts)
}

func (t *parsedTurn) addOrUpdateTool(callID, content, status string, meta map[string]any, ts int64) {
	if t == nil {
		return
	}
	callID = strings.TrimSpace(callID)
	if callID == "" {
		callID = fmt.Sprintf("tool:%d", t.nextSeq)
	}
	entryID := "tool:" + callID
	if idx, ok := t.itemIndexByEntry[entryID]; ok {
		entry := t.items[idx]
		if strings.TrimSpace(content) != "" {
			entry.ContentText = content
		}
		if merged := mergeMeta(entry.Meta, meta); len(merged) > 0 {
			entry.Meta = merged
		}
		if strings.TrimSpace(status) != "" {
			entry.Status = status
		}
		entry.UpdatedAt = chooseImportedTime(entry.UpdatedAt, ts)
		t.items[idx] = entry
		t.lastUpdatedAt = chooseImportedTime(t.lastUpdatedAt, ts)
		return
	}
	t.addEntry(entryID, "tool", defaultToolStatus(status), content, meta, ts)
}

func (t *parsedTurn) addEntry(entryID, kind, status, content string, meta map[string]any, ts int64) {
	if t == nil {
		return
	}
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		entryID = fmt.Sprintf("%s:%d", kind, t.nextSeq)
	}
	entry := store.ImportedChatTurnItem{
		EntryID:     entryID,
		Seq:         t.nextSeq,
		Kind:        strings.TrimSpace(kind),
		Status:      strings.TrimSpace(status),
		ContentText: strings.TrimSpace(content),
		Meta:        cloneMeta(meta),
		CreatedAt:   ts,
		UpdatedAt:   ts,
	}
	t.itemIndexByEntry[entryID] = len(t.items)
	t.items = append(t.items, entry)
	t.nextSeq += 1
	t.lastUpdatedAt = chooseImportedTime(t.lastUpdatedAt, ts)
}

func payloadType(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func extractEventMessage(payload map[string]any) string {
	return strings.TrimSpace(stringField(payload, "message"))
}

func compactAssistantMessage(payload map[string]any) string {
	if text := strings.TrimSpace(stringField(payload, "last_agent_message")); text != "" {
		return text
	}
	return strings.TrimSpace(stringField(payload, "message"))
}

func extractMessageContent(payload map[string]any) string {
	items, _ := payload["content"].([]any)
	parts := make([]string, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		itemType := strings.TrimSpace(stringField(entry, "type"))
		if itemType != "output_text" && itemType != "input_text" {
			continue
		}
		text := strings.TrimSpace(stringField(entry, "text"))
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func extractReasoningText(payload map[string]any) string {
	summary, _ := payload["summary"].([]any)
	parts := make([]string, 0, len(summary))
	for _, item := range summary {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringField(entry, "type")) != "summary_text" {
			continue
		}
		text := strings.TrimSpace(stringField(entry, "text"))
		if text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) > 0 {
		return strings.TrimSpace(strings.Join(parts, "\n\n"))
	}
	if text := strings.TrimSpace(stringField(payload, "content")); text != "" {
		return text
	}
	return ""
}

func toolContentFromCall(payload map[string]any) string {
	name := strings.TrimSpace(stringField(payload, "name"))
	args := parseJSONStringMap(stringField(payload, "arguments"))
	switch name {
	case "exec_command":
		if cmd := strings.TrimSpace(stringField(args, "cmd")); cmd != "" {
			return truncateToolLabel(cmd)
		}
	}
	if query := firstNonEmpty(stringField(args, "query"), stringField(args, "q")); strings.TrimSpace(query) != "" {
		return truncateToolLabel(name + ": " + query)
	}
	if name == "" {
		return "tool call"
	}
	return truncateToolLabel(name)
}

func toolContentFromCustomCall(payload map[string]any) string {
	name := strings.TrimSpace(stringField(payload, "name"))
	if name == "" {
		name = "custom tool"
	}
	return truncateToolLabel(name)
}

func toolContentFromWebSearch(payload map[string]any) string {
	action, _ := payload["action"].(map[string]any)
	query := strings.TrimSpace(firstNonEmpty(stringField(action, "query"), stringField(payload, "query")))
	if query == "" {
		return "web search"
	}
	return truncateToolLabel("web search: " + query)
}

func parseFunctionCallOutput(payload map[string]any) map[string]any {
	raw := strings.TrimSpace(stringField(payload, "output"))
	if raw == "" {
		return nil
	}
	meta := map[string]any{}
	if match := reExitCode.FindStringSubmatch(raw); len(match) > 1 {
		if code, err := strconv.Atoi(match[1]); err == nil {
			meta["exit_code"] = code
		}
	}
	if idx := strings.LastIndex(raw, "\nOutput:\n"); idx >= 0 {
		stdout := strings.TrimSpace(raw[idx+len("\nOutput:\n"):])
		if stdout != "" {
			meta["stdout"] = stdout
		}
	} else {
		meta["stdout"] = raw
	}
	return meta
}

func parseCustomToolCallOutput(payload map[string]any) map[string]any {
	raw := strings.TrimSpace(stringField(payload, "output"))
	if raw == "" {
		return nil
	}
	meta := map[string]any{}
	var decoded struct {
		Output   string `json:"output"`
		Metadata struct {
			ExitCode any `json:"exit_code"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
		if text := strings.TrimSpace(decoded.Output); text != "" {
			meta["stdout"] = text
		}
		if exitCode, ok := numericValue(decoded.Metadata.ExitCode); ok {
			meta["exit_code"] = exitCode
		}
		return meta
	}
	meta["stdout"] = raw
	return meta
}

func toolStatusFromMeta(meta map[string]any) string {
	if len(meta) == 0 {
		return "done"
	}
	if exitCode, ok := numericValue(meta["exit_code"]); ok && exitCode != 0 {
		return "failed"
	}
	return "success"
}

func defaultToolStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "created"
	}
	return status
}

func parseJSONStringMap(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func parseEventTime(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return 0
	}
	return parsed.UnixMilli()
}

func stringField(payload map[string]any, key string) string {
	if len(payload) == 0 {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func numericValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	case json.Number:
		v, err := typed.Int64()
		return v, err == nil
	default:
		return 0, false
	}
}

func mergeMeta(left, right map[string]any) map[string]any {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	out := map[string]any{}
	for key, value := range left {
		out[key] = value
	}
	for key, value := range right {
		out[key] = value
	}
	return out
}

func cloneMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]any, len(meta))
	for key, value := range meta {
		out[key] = value
	}
	return out
}

func normalizeIDList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func importedProviderFrom(modelProvider string) string {
	switch strings.ToLower(strings.TrimSpace(modelProvider)) {
	case "anthropic":
		return "anthropic"
	case "cli":
		return "cli"
	default:
		return defaultImportedProvider
	}
}

func chooseImportedTime(primary, fallback int64) int64 {
	primary = normalizeImportedTime(primary)
	fallback = normalizeImportedTime(fallback)
	if primary > 0 {
		return primary
	}
	return fallback
}

func normalizeImportedTime(value int64) int64 {
	if value <= 0 {
		return 0
	}
	if value < 1_000_000_000_000 {
		return value * 1000
	}
	return value
}

func firstCreatedAt(messages []store.ImportedChatMessage, turns []store.ImportedChatTurn) int64 {
	minValue := int64(0)
	for _, msg := range messages {
		ts := normalizeImportedTime(msg.CreatedAt)
		if ts <= 0 {
			continue
		}
		if minValue == 0 || ts < minValue {
			minValue = ts
		}
	}
	for _, turn := range turns {
		ts := normalizeImportedTime(turn.CreatedAt)
		if ts <= 0 {
			continue
		}
		if minValue == 0 || ts < minValue {
			minValue = ts
		}
	}
	return minValue
}

func maxImportedTurn(messages []store.ImportedChatMessage, turns []store.ImportedChatTurn) int64 {
	maxTurn := int64(0)
	for _, msg := range messages {
		if msg.Turn > maxTurn {
			maxTurn = msg.Turn
		}
	}
	for _, turn := range turns {
		if turn.Turn > maxTurn {
			maxTurn = turn.Turn
		}
	}
	return maxTurn
}

func defaultWorkspace(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "."
	}
	return path
}

func truncateToolLabel(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) > 160 {
		return strings.TrimSpace(string(runes[:160])) + "…"
	}
	return text
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
