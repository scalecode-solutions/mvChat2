// Package irido provides the message content format for mvChat2.
// Irido is named after iridophores - the reflecting cells in octopus skin
// that create hidden iridescent colors ("hidden beauty").
//
// Structure: { v: 1, text?: string, media?: [], reply?: {}, mentions?: [] }
// MIME type: application/x-irido
package irido

import (
	"encoding/json"
	"errors"
	"strings"
)

var (
	ErrInvalidContent = errors.New("invalid irido content")
	ErrNotIrido       = errors.New("content is not irido format")
)

// Irido represents the root message content structure.
type Irido struct {
	// Version - always 1 for now
	V int `json:"v"`
	// Text content with Markdown support
	Text string `json:"text,omitempty"`
	// Media attachments (max 10)
	Media []Media `json:"media,omitempty"`
	// Reply to another message
	Reply *Reply `json:"reply,omitempty"`
	// User mentions
	Mentions []Mention `json:"mentions,omitempty"`
}

// Media represents a media attachment.
type Media struct {
	// Type: image, video, audio, file, embed
	Type string `json:"type"`
	// File ID (UUID reference to files table)
	Ref string `json:"ref,omitempty"`
	// Original filename
	Name string `json:"name,omitempty"`
	// MIME type
	Mime string `json:"mime,omitempty"`
	// File size in bytes
	Size int64 `json:"size,omitempty"`
	// Image/Video dimensions
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
	// Audio/Video duration in seconds
	Duration float64 `json:"duration,omitempty"`
	// Embed data for link previews
	Embed *Embed `json:"embed,omitempty"`
}

// Embed represents link preview data.
type Embed struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Thumbnail   string `json:"thumbnail,omitempty"`
	SiteName    string `json:"siteName,omitempty"`
	EmbedType   string `json:"embedType,omitempty"` // article, video, photo, rich
}

// Reply represents a reply to another message.
type Reply struct {
	// Sequence number of the message being replied to
	Seq int `json:"seq"`
	// Text preview of the original message (truncated)
	Preview string `json:"preview,omitempty"`
	// Original sender's user ID
	From string `json:"from,omitempty"`
}

// Mention represents a user mention in the text.
type Mention struct {
	// User ID of the mentioned user (UUID)
	UserID string `json:"userId"`
	// Display name at time of mention
	Username string `json:"username"`
	// Character offset in text (in graphemes, not bytes)
	Offset int `json:"offset"`
	// Length of the mention text (in graphemes)
	Length int `json:"length"`
}

// New creates a new Irido with just text content.
func New(text string) *Irido {
	return &Irido{V: 1, Text: text}
}

// NewWithMedia creates a new Irido with text and media.
func NewWithMedia(text string, media []Media) *Irido {
	return &Irido{V: 1, Text: text, Media: media}
}

// IsIrido checks if content is in Irido format (has v:1).
func IsIrido(content any) bool {
	if content == nil {
		return false
	}

	switch c := content.(type) {
	case map[string]any:
		if v, ok := c["v"]; ok {
			switch vv := v.(type) {
			case int:
				return vv == 1
			case float64:
				return vv == 1
			}
		}
	case *Irido:
		return c.V == 1
	case Irido:
		return c.V == 1
	}
	return false
}

// Parse converts raw content to an Irido struct.
// Handles: plain strings, JSON bytes, map[string]any, and Irido structs.
func Parse(content any) (*Irido, error) {
	if content == nil {
		return nil, nil
	}

	switch c := content.(type) {
	case string:
		// Plain text string - wrap in Irido
		return &Irido{V: 1, Text: c}, nil

	case []byte:
		// JSON bytes - unmarshal
		var irido Irido
		if err := json.Unmarshal(c, &irido); err != nil {
			// Try as plain text
			return &Irido{V: 1, Text: string(c)}, nil
		}
		if irido.V != 1 {
			return nil, ErrInvalidContent
		}
		return &irido, nil

	case *Irido:
		return c, nil

	case Irido:
		return &c, nil

	case map[string]any:
		return parseMap(c)

	default:
		return nil, ErrInvalidContent
	}
}

func parseMap(c map[string]any) (*Irido, error) {
	v, ok := c["v"]
	if !ok {
		return nil, ErrNotIrido
	}

	var version int
	switch vv := v.(type) {
	case int:
		version = vv
	case float64:
		version = int(vv)
	default:
		return nil, ErrInvalidContent
	}
	if version != 1 {
		return nil, ErrInvalidContent
	}

	irido := &Irido{V: 1}

	if text, ok := c["text"].(string); ok {
		irido.Text = text
	}

	if media, ok := c["media"].([]any); ok {
		for _, m := range media {
			if mm, ok := m.(map[string]any); ok {
				parsed := parseMedia(mm)
				if parsed != nil {
					irido.Media = append(irido.Media, *parsed)
				}
			}
		}
	}

	if reply, ok := c["reply"].(map[string]any); ok {
		irido.Reply = parseReply(reply)
	}

	if mentions, ok := c["mentions"].([]any); ok {
		for _, m := range mentions {
			if mm, ok := m.(map[string]any); ok {
				parsed := parseMention(mm)
				if parsed != nil {
					irido.Mentions = append(irido.Mentions, *parsed)
				}
			}
		}
	}

	return irido, nil
}

func parseMedia(m map[string]any) *Media {
	media := &Media{}

	if t, ok := m["type"].(string); ok {
		media.Type = t
	}
	if ref, ok := m["ref"].(string); ok {
		media.Ref = ref
	}
	if name, ok := m["name"].(string); ok {
		media.Name = name
	}
	if mime, ok := m["mime"].(string); ok {
		media.Mime = mime
	}
	if size, ok := m["size"].(float64); ok {
		media.Size = int64(size)
	}
	if width, ok := m["width"].(float64); ok {
		media.Width = int(width)
	}
	if height, ok := m["height"].(float64); ok {
		media.Height = int(height)
	}
	if duration, ok := m["duration"].(float64); ok {
		media.Duration = duration
	}

	if embed, ok := m["embed"].(map[string]any); ok {
		media.Embed = parseEmbed(embed)
	}

	return media
}

func parseEmbed(e map[string]any) *Embed {
	embed := &Embed{}

	if url, ok := e["url"].(string); ok {
		embed.URL = url
	}
	if title, ok := e["title"].(string); ok {
		embed.Title = title
	}
	if desc, ok := e["description"].(string); ok {
		embed.Description = desc
	}
	if thumb, ok := e["thumbnail"].(string); ok {
		embed.Thumbnail = thumb
	}
	if site, ok := e["siteName"].(string); ok {
		embed.SiteName = site
	}
	if et, ok := e["embedType"].(string); ok {
		embed.EmbedType = et
	}

	return embed
}

func parseReply(r map[string]any) *Reply {
	reply := &Reply{}

	if seq, ok := r["seq"].(float64); ok {
		reply.Seq = int(seq)
	}
	if preview, ok := r["preview"].(string); ok {
		reply.Preview = preview
	}
	if from, ok := r["from"].(string); ok {
		reply.From = from
	}

	return reply
}

func parseMention(m map[string]any) *Mention {
	mention := &Mention{}

	if userId, ok := m["userId"].(string); ok {
		mention.UserID = userId
	}
	if username, ok := m["username"].(string); ok {
		mention.Username = username
	}
	if offset, ok := m["offset"].(float64); ok {
		mention.Offset = int(offset)
	}
	if length, ok := m["length"].(float64); ok {
		mention.Length = int(length)
	}

	return mention
}

// ToJSON serializes Irido to JSON bytes.
func (i *Irido) ToJSON() ([]byte, error) {
	return json.Marshal(i)
}

// String returns JSON string representation.
func (i *Irido) String() string {
	data, _ := json.Marshal(i)
	return string(data)
}

// PlainText converts Irido content to plain text for search/notifications.
// Markdown formatting is preserved. Media is represented as [TYPE 'name'].
func PlainText(content any) (string, error) {
	irido, err := Parse(content)
	if err != nil {
		return "", err
	}
	if irido == nil {
		return "", nil
	}

	var parts []string

	if irido.Text != "" {
		parts = append(parts, irido.Text)
	}

	for _, m := range irido.Media {
		desc := mediaDescription(&m)
		if desc != "" {
			parts = append(parts, desc)
		}
	}

	return strings.TrimSpace(strings.Join(parts, " ")), nil
}

func mediaDescription(m *Media) string {
	typeNames := map[string]string{
		"image": "IMAGE",
		"video": "VIDEO",
		"audio": "AUDIO",
		"file":  "FILE",
		"embed": "LINK",
	}

	typeName := typeNames[m.Type]
	if typeName == "" {
		typeName = strings.ToUpper(m.Type)
	}

	name := m.Name
	if name == "" && m.Embed != nil {
		name = m.Embed.Title
		if name == "" {
			name = m.Embed.URL
		}
	}
	if name == "" {
		name = "attachment"
	}

	return "[" + typeName + " '" + name + "']"
}

// Preview creates a shortened version for push notifications.
// maxLength is in graphemes (not bytes) to handle emoji correctly.
func Preview(content any, maxLength int) (string, error) {
	irido, err := Parse(content)
	if err != nil {
		return "", err
	}
	if irido == nil {
		return "", nil
	}

	var result string

	if irido.Text != "" {
		g := NewGraphemes(irido.Text)
		if g.Length() > maxLength {
			result = g.Slice(0, maxLength) + "…"
		} else {
			result = irido.Text
		}
	}

	// If no text but has media, describe the first media item
	if result == "" && len(irido.Media) > 0 {
		result = mediaDescription(&irido.Media[0])
	}

	return strings.TrimSpace(result), nil
}

// PreviewIrido creates a shortened Irido for push notifications.
func PreviewIrido(content any, maxLength int) (*Irido, error) {
	irido, err := Parse(content)
	if err != nil {
		return nil, err
	}
	if irido == nil {
		return nil, nil
	}

	preview := &Irido{V: 1}

	// Truncate text
	if irido.Text != "" {
		g := NewGraphemes(irido.Text)
		if g.Length() > maxLength {
			preview.Text = g.Slice(0, maxLength) + "…"
		} else {
			preview.Text = irido.Text
		}
	}

	// Include first media item with minimal data
	if len(irido.Media) > 0 {
		m := irido.Media[0]
		preview.Media = []Media{{
			Type:   m.Type,
			Ref:    m.Ref,
			Name:   m.Name,
			Mime:   m.Mime,
			Width:  m.Width,
			Height: m.Height,
		}}
	}

	// Include reply reference
	if irido.Reply != nil {
		g := NewGraphemes(irido.Reply.Preview)
		previewText := irido.Reply.Preview
		if g.Length() > 50 {
			previewText = g.Slice(0, 50) + "…"
		}
		preview.Reply = &Reply{
			Seq:     irido.Reply.Seq,
			Preview: previewText,
			From:    irido.Reply.From,
		}
	}

	return preview, nil
}

// GetFileRefs extracts all file references from media attachments.
func GetFileRefs(content any) []string {
	irido, err := Parse(content)
	if err != nil || irido == nil {
		return nil
	}

	var refs []string
	for _, m := range irido.Media {
		if m.Ref != "" {
			refs = append(refs, m.Ref)
		}
	}
	return refs
}

// GetMentionedUsers extracts all mentioned user IDs.
func GetMentionedUsers(content any) []string {
	irido, err := Parse(content)
	if err != nil || irido == nil {
		return nil
	}

	var users []string
	for _, m := range irido.Mentions {
		if m.UserID != "" {
			users = append(users, m.UserID)
		}
	}
	return users
}

// HasMedia returns true if the content has any media attachments.
func HasMedia(content any) bool {
	irido, err := Parse(content)
	if err != nil || irido == nil {
		return false
	}
	return len(irido.Media) > 0
}

// IsReply returns true if the content is a reply to another message.
func IsReply(content any) bool {
	irido, err := Parse(content)
	if err != nil || irido == nil {
		return false
	}
	return irido.Reply != nil && irido.Reply.Seq > 0
}

// Validate checks if the Irido content is valid.
func Validate(content any) error {
	irido, err := Parse(content)
	if err != nil {
		return err
	}
	if irido == nil {
		return ErrInvalidContent
	}

	// Must have text or media
	if irido.Text == "" && len(irido.Media) == 0 {
		return errors.New("irido: must have text or media")
	}

	// Max 10 media attachments
	if len(irido.Media) > 10 {
		return errors.New("irido: max 10 media attachments")
	}

	// Validate mentions
	if len(irido.Mentions) > 0 && irido.Text == "" {
		return errors.New("irido: mentions require text")
	}

	return nil
}
