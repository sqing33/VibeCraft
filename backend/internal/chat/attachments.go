package chat

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	openai "github.com/openai/openai-go"
	openai_responses "github.com/openai/openai-go/responses"

	"vibecraft/backend/internal/id"
	"vibecraft/backend/internal/paths"
	"vibecraft/backend/internal/store"
)

const (
	attachmentOnlyDisplayText = "（仅附件）"
	attachmentOnlyModelPrompt = "请先阅读这些附件，我稍后会继续提问。"

	maxAttachmentCount       = 5
	maxAttachmentTotalBytes  = 20 << 20
	maxBinaryAttachmentBytes = 10 << 20
	maxTextAttachmentBytes   = 2 << 20
)

type UploadedAttachment struct {
	FileName string
	MIMEType string
	Bytes    []byte
}

type preparedAttachment struct {
	FileName  string
	MIMEType  string
	Kind      string
	SizeBytes int64
	Bytes     []byte
}

func PrepareTurnInputs(rawInput string, attachmentCount int) (userText string, providerInput string) {
	return defaultTurnInputs(rawInput, attachmentCount)
}

func defaultTurnInputs(rawInput string, attachmentCount int) (userText string, providerInput string) {
	trimmed := strings.TrimSpace(rawInput)
	if trimmed != "" {
		return trimmed, trimmed
	}
	if attachmentCount > 0 {
		return attachmentOnlyDisplayText, attachmentOnlyModelPrompt
	}
	return "", ""
}

func prepareUploadedAttachments(uploads []UploadedAttachment) ([]preparedAttachment, error) {
	if len(uploads) == 0 {
		return nil, nil
	}
	if len(uploads) > maxAttachmentCount {
		return nil, fmt.Errorf("%w: too many attachments (max %d)", store.ErrValidation, maxAttachmentCount)
	}
	prepared := make([]preparedAttachment, 0, len(uploads))
	var totalBytes int64
	for _, upload := range uploads {
		fileName := sanitizeAttachmentFileName(upload.FileName)
		if fileName == "" {
			return nil, fmt.Errorf("%w: attachment filename is required", store.ErrValidation)
		}
		if len(upload.Bytes) == 0 {
			return nil, fmt.Errorf("%w: attachment %q is empty", store.ErrValidation, fileName)
		}
		mimeType, kind, err := detectAttachmentType(fileName, upload.MIMEType, upload.Bytes)
		if err != nil {
			return nil, err
		}
		sizeBytes := int64(len(upload.Bytes))
		totalBytes += sizeBytes
		if totalBytes > maxAttachmentTotalBytes {
			return nil, fmt.Errorf("%w: attachments exceed total size limit (%d bytes)", store.ErrValidation, maxAttachmentTotalBytes)
		}
		switch kind {
		case store.ChatAttachmentKindText:
			if sizeBytes > maxTextAttachmentBytes {
				return nil, fmt.Errorf("%w: text attachment %q exceeds %d bytes", store.ErrValidation, fileName, maxTextAttachmentBytes)
			}
		default:
			if sizeBytes > maxBinaryAttachmentBytes {
				return nil, fmt.Errorf("%w: attachment %q exceeds %d bytes", store.ErrValidation, fileName, maxBinaryAttachmentBytes)
			}
		}
		prepared = append(prepared, preparedAttachment{
			FileName:  fileName,
			MIMEType:  mimeType,
			Kind:      kind,
			SizeBytes: sizeBytes,
			Bytes:     upload.Bytes,
		})
	}
	return prepared, nil
}

func sanitizeAttachmentFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	base := filepath.Base(name)
	base = strings.ReplaceAll(base, string(filepath.Separator), "_")
	base = strings.TrimSpace(base)
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return base
}

func detectAttachmentType(fileName, declaredMIME string, data []byte) (mimeType string, kind string, err error) {
	declaredMIME = normalizeMIME(declaredMIME)
	ext := strings.ToLower(filepath.Ext(fileName))
	sniffed := normalizeMIME(http.DetectContentType(data))

	if mt := pickImageMIME(ext, declaredMIME, sniffed); mt != "" {
		return mt, store.ChatAttachmentKindImage, nil
	}
	if ext == ".pdf" || declaredMIME == "application/pdf" || sniffed == "application/pdf" {
		return "application/pdf", store.ChatAttachmentKindPDF, nil
	}
	if isSupportedTextAttachment(ext, declaredMIME, sniffed, data) {
		mt := declaredMIME
		if mt == "" {
			mt = mime.TypeByExtension(ext)
		}
		mt = normalizeMIME(mt)
		if mt == "" || mt == "application/octet-stream" {
			mt = "text/plain"
		}
		return mt, store.ChatAttachmentKindText, nil
	}
	return "", "", fmt.Errorf("%w: unsupported attachment type for %q", store.ErrValidation, fileName)
}

func pickImageMIME(ext, declaredMIME, sniffed string) string {
	allowedByExt := map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
	}
	allowed := map[string]struct{}{
		"image/png":  {},
		"image/jpeg": {},
		"image/gif":  {},
		"image/webp": {},
	}
	if mt, ok := allowedByExt[ext]; ok {
		if sniffed == "" || sniffed == mt || strings.HasPrefix(sniffed, "image/") {
			return mt
		}
	}
	if _, ok := allowed[declaredMIME]; ok {
		return declaredMIME
	}
	if _, ok := allowed[sniffed]; ok {
		return sniffed
	}
	return ""
}

func isSupportedTextAttachment(ext, declaredMIME, sniffed string, data []byte) bool {
	if strings.HasPrefix(declaredMIME, "text/") || strings.HasPrefix(sniffed, "text/") {
		return true
	}
	allowedExts := map[string]struct{}{
		".txt": {}, ".md": {}, ".markdown": {}, ".json": {}, ".yaml": {}, ".yml": {},
		".ts": {}, ".tsx": {}, ".js": {}, ".jsx": {}, ".mjs": {}, ".cjs": {},
		".go": {}, ".py": {}, ".sh": {}, ".bash": {}, ".zsh": {}, ".sql": {},
		".html": {}, ".css": {}, ".scss": {}, ".less": {}, ".xml": {}, ".toml": {},
		".ini": {}, ".env": {}, ".java": {}, ".rb": {}, ".rs": {}, ".c": {}, ".h": {},
		".cpp": {}, ".hpp": {}, ".cs": {}, ".php": {}, ".swift": {}, ".kt": {}, ".kts": {},
	}
	if _, ok := allowedExts[ext]; ok {
		return true
	}
	if !utf8.Valid(data) {
		return false
	}
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}

func normalizeMIME(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(v)
	if err == nil {
		return strings.ToLower(strings.TrimSpace(mediaType))
	}
	return strings.ToLower(strings.TrimSpace(strings.Split(v, ";")[0]))
}

func (m *Manager) persistTurnAttachments(ctx context.Context, userMsg store.ChatMessage, uploads []UploadedAttachment) ([]store.ChatAttachment, error) {
	prepared, err := prepareUploadedAttachments(uploads)
	if err != nil {
		return nil, err
	}
	if len(prepared) == 0 {
		return nil, nil
	}
	messageDir, err := paths.ChatAttachmentMessageDir(userMsg.SessionID, userMsg.ID)
	if err != nil {
		return nil, fmt.Errorf("resolve chat attachment dir: %w", err)
	}
	if err := paths.EnsureDir(messageDir); err != nil {
		return nil, fmt.Errorf("ensure chat attachment dir: %w", err)
	}
	attachmentsRoot, err := paths.ChatAttachmentsDir()
	if err != nil {
		return nil, fmt.Errorf("resolve chat attachments dir: %w", err)
	}
	usedNames := make(map[string]int)
	createdFiles := make([]string, 0, len(prepared))
	attachments := make([]store.ChatAttachment, 0, len(prepared))
	for _, item := range prepared {
		fileName := uniquifyAttachmentFileName(item.FileName, usedNames)
		fullPath := filepath.Join(messageDir, fileName)
		if err := os.WriteFile(fullPath, item.Bytes, 0o644); err != nil {
			cleanupAttachmentFiles(createdFiles)
			return nil, fmt.Errorf("write chat attachment %q: %w", fileName, err)
		}
		createdFiles = append(createdFiles, fullPath)
		relPath, err := filepath.Rel(attachmentsRoot, fullPath)
		if err != nil {
			cleanupAttachmentFiles(createdFiles)
			return nil, fmt.Errorf("resolve chat attachment relative path: %w", err)
		}
		attachments = append(attachments, store.ChatAttachment{
			ID:             id.New("ca_"),
			SessionID:      userMsg.SessionID,
			MessageID:      userMsg.ID,
			Kind:           item.Kind,
			FileName:       fileName,
			MIMEType:       item.MIMEType,
			SizeBytes:      item.SizeBytes,
			StorageRelPath: filepath.ToSlash(relPath),
			CreatedAt:      userMsg.CreatedAt,
		})
	}
	if err := m.store.CreateChatAttachments(ctx, store.CreateChatAttachmentsParams{Attachments: attachments}); err != nil {
		cleanupAttachmentFiles(createdFiles)
		return nil, err
	}
	return attachments, nil
}

func uniquifyAttachmentFileName(name string, used map[string]int) string {
	name = sanitizeAttachmentFileName(name)
	if name == "" {
		name = "attachment"
	}
	count := used[name]
	used[name] = count + 1
	if count == 0 {
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s_%d%s", base, count+1, ext)
}

func cleanupAttachmentFiles(pathsToRemove []string) {
	for i := len(pathsToRemove) - 1; i >= 0; i-- {
		_ = os.Remove(pathsToRemove[i])
	}
}

func attachmentDiskPath(relPath string) (string, error) {
	root, err := paths.ChatAttachmentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.FromSlash(relPath)), nil
}

func readAttachmentBytes(att store.ChatAttachment) ([]byte, error) {
	fullPath, err := attachmentDiskPath(att.StorageRelPath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read attachment %q: %w", att.FileName, err)
	}
	return data, nil
}

func anyMessageHasAttachments(messages []store.ChatMessage) bool {
	for _, msg := range messages {
		if len(msg.Attachments) > 0 {
			return true
		}
	}
	return false
}

func buildCurrentInputDebug(modelInput string, attachments []store.ChatAttachment) string {
	parts := make([]string, 0, 2)
	if trimmed := strings.TrimSpace(modelInput); trimmed != "" {
		parts = append(parts, "Current user input:\n"+trimmed)
	}
	if len(attachments) > 0 {
		parts = append(parts, "Attachments:\n"+strings.Join(attachmentDebugLines(attachments), "\n"))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func renderConversationDebug(summary *string, messages []store.ChatMessage, keepRecent int) string {
	messages = filterModelContextMessages(messages)
	parts := make([]string, 0, 4)
	if s := strings.TrimSpace(stringOrEmpty(summary)); s != "" {
		parts = append(parts, "Session summary:\n"+s)
	}
	if keepRecent <= 0 {
		keepRecent = len(messages)
	}
	if len(messages) > keepRecent {
		messages = messages[len(messages)-keepRecent:]
	}
	if len(messages) > 0 {
		var b strings.Builder
		b.WriteString("Recent conversation:\n")
		for _, msg := range messages {
			b.WriteString(strings.ToUpper(msg.Role))
			b.WriteString(": ")
			label := strings.TrimSpace(msg.ContentText)
			if label == "" {
				label = attachmentOnlyDisplayText
			}
			b.WriteString(label)
			if len(msg.Attachments) > 0 {
				b.WriteString(" [")
				b.WriteString(strings.Join(attachmentDebugLabels(msg.Attachments), ", "))
				b.WriteString("]")
			}
			b.WriteString("\n")
		}
		parts = append(parts, strings.TrimSpace(b.String()))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func attachmentDebugLines(attachments []store.ChatAttachment) []string {
	labels := attachmentDebugLabels(attachments)
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		out = append(out, "- "+label)
	}
	return out
}

func attachmentDebugLabels(attachments []store.ChatAttachment) []string {
	labels := make([]string, 0, len(attachments))
	for _, att := range attachments {
		labels = append(labels, fmt.Sprintf("%s (%s)", att.FileName, att.Kind))
	}
	sort.Strings(labels)
	return labels
}

func buildOpenAIMessageContent(text string, attachments []store.ChatAttachment) (openai_responses.ResponseInputMessageContentListParam, error) {
	content := make(openai_responses.ResponseInputMessageContentListParam, 0, len(attachments)+1)
	trimmed := strings.TrimSpace(text)
	if trimmed != "" && trimmed != attachmentOnlyDisplayText {
		content = append(content, openai_responses.ResponseInputContentParamOfInputText(trimmed))
	}
	for _, att := range attachments {
		data, err := readAttachmentBytes(att)
		if err != nil {
			return nil, err
		}
		switch att.Kind {
		case store.ChatAttachmentKindImage:
			content = append(content, openai_responses.ResponseInputContentUnionParam{
				OfInputImage: &openai_responses.ResponseInputImageParam{
					Detail:   openai_responses.ResponseInputImageDetailAuto,
					ImageURL: openai.String(fmt.Sprintf("data:%s;base64,%s", att.MIMEType, base64.StdEncoding.EncodeToString(data))),
				},
			})
		case store.ChatAttachmentKindPDF:
			content = append(content, openai_responses.ResponseInputContentUnionParam{
				OfInputFile: &openai_responses.ResponseInputFileParam{
					FileData: openai.String(base64.StdEncoding.EncodeToString(data)),
					Filename: openai.String(att.FileName),
				},
			})
		case store.ChatAttachmentKindText:
			content = append(content, openai_responses.ResponseInputContentParamOfInputText(formatTextAttachment(att.FileName, data)))
		}
	}
	if len(content) == 0 {
		content = append(content, openai_responses.ResponseInputContentParamOfInputText(attachmentOnlyModelPrompt))
	}
	return content, nil
}

func buildAnthropicMessageBlocks(text string, attachments []store.ChatAttachment) ([]anthropic.ContentBlockParamUnion, error) {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(attachments)+1)
	trimmed := strings.TrimSpace(text)
	if trimmed != "" && trimmed != attachmentOnlyDisplayText {
		blocks = append(blocks, anthropic.NewTextBlock(trimmed))
	}
	for _, att := range attachments {
		data, err := readAttachmentBytes(att)
		if err != nil {
			return nil, err
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		switch att.Kind {
		case store.ChatAttachmentKindImage:
			blocks = append(blocks, anthropic.NewImageBlockBase64(att.MIMEType, b64))
		case store.ChatAttachmentKindPDF:
			blocks = append(blocks, anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{Data: b64}))
		case store.ChatAttachmentKindText:
			blocks = append(blocks, anthropic.NewTextBlock(formatTextAttachment(att.FileName, data)))
		}
	}
	if len(blocks) == 0 {
		blocks = append(blocks, anthropic.NewTextBlock(attachmentOnlyModelPrompt))
	}
	return blocks, nil
}

func formatTextAttachment(name string, data []byte) string {
	return fmt.Sprintf("Attachment: %s\n%s", name, strings.TrimSpace(string(data)))
}
