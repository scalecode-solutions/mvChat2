// Package redis provides Redis client and pub/sub for multi-instance messaging.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client with mvChat2-specific operations.
type Client struct {
	rdb    *redis.Client
	nodeID string // Unique identifier for this server instance
	prefix string // Key prefix for namespacing
}

// Config holds Redis connection settings.
type Config struct {
	Addr     string // host:port
	Password string
	DB       int
	NodeID   string // Unique ID for this instance (hostname, UUID, etc.)
	Prefix   string // Key prefix (default: "mvchat2:")
}

// New creates a new Redis client.
func New(cfg Config) (*Client, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "mvchat2:"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Client{
		rdb:    rdb,
		nodeID: cfg.NodeID,
		prefix: cfg.Prefix,
	}, nil
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// NodeID returns this instance's node ID.
func (c *Client) NodeID() string {
	return c.nodeID
}

// key prefixes a key with the namespace.
func (c *Client) key(k string) string {
	return c.prefix + k
}

// ============================================================================
// Presence Cache
// ============================================================================

// SetOnline marks a user as online on this node.
func (c *Client) SetOnline(ctx context.Context, userID string) error {
	key := c.key("online:" + userID)
	// Store node ID with 5 minute TTL (refreshed periodically)
	return c.rdb.Set(ctx, key, c.nodeID, 5*time.Minute).Err()
}

// SetOffline removes a user's online status.
func (c *Client) SetOffline(ctx context.Context, userID string) error {
	key := c.key("online:" + userID)
	return c.rdb.Del(ctx, key).Err()
}

// IsOnline checks if a user is online (on any node).
func (c *Client) IsOnline(ctx context.Context, userID string) (bool, error) {
	key := c.key("online:" + userID)
	exists, err := c.rdb.Exists(ctx, key).Result()
	return exists > 0, err
}

// GetOnlineNode returns which node a user is connected to (empty if offline).
func (c *Client) GetOnlineNode(ctx context.Context, userID string) (string, error) {
	key := c.key("online:" + userID)
	node, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return node, err
}

// RefreshOnline extends the TTL for a user's online status.
func (c *Client) RefreshOnline(ctx context.Context, userID string) error {
	key := c.key("online:" + userID)
	return c.rdb.Expire(ctx, key, 5*time.Minute).Err()
}

// ============================================================================
// Pub/Sub
// ============================================================================

// Message represents a pub/sub message.
type Message struct {
	Type     string          `json:"type"`    // "data", "info", "pres"
	FromNode string          `json:"from"`    // Originating node ID
	Payload  json.RawMessage `json:"payload"` // The actual message
}

// PubSub handles pub/sub operations.
type PubSub struct {
	client  *Client
	pubsub  *redis.PubSub
	handler func(msg *Message)
}

// NewPubSub creates a new pub/sub handler.
func (c *Client) NewPubSub(handler func(msg *Message)) *PubSub {
	return &PubSub{
		client:  c,
		handler: handler,
	}
}

// Subscribe subscribes to channels for receiving messages.
func (ps *PubSub) Subscribe(ctx context.Context, channels ...string) error {
	prefixed := make([]string, len(channels))
	for i, ch := range channels {
		prefixed[i] = ps.client.key("ch:" + ch)
	}

	ps.pubsub = ps.client.rdb.Subscribe(ctx, prefixed...)
	return nil
}

// SubscribeToNode subscribes to this node's direct channel.
func (ps *PubSub) SubscribeToNode(ctx context.Context) error {
	channel := ps.client.key("node:" + ps.client.nodeID)
	ps.pubsub = ps.client.rdb.Subscribe(ctx, channel)
	return nil
}

// Listen starts listening for messages (blocking).
func (ps *PubSub) Listen(ctx context.Context) {
	if ps.pubsub == nil {
		return
	}

	ch := ps.pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case redisMsg, ok := <-ch:
			if !ok {
				return
			}
			var msg Message
			if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
				continue
			}
			// Skip messages from self
			if msg.FromNode == ps.client.nodeID {
				continue
			}
			if ps.handler != nil {
				ps.handler(&msg)
			}
		}
	}
}

// Close closes the pub/sub connection.
func (ps *PubSub) Close() error {
	if ps.pubsub != nil {
		return ps.pubsub.Close()
	}
	return nil
}

// Publish publishes a message to a channel.
func (c *Client) Publish(ctx context.Context, channel string, msgType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msg := Message{
		Type:     msgType,
		FromNode: c.nodeID,
		Payload:  data,
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.rdb.Publish(ctx, c.key("ch:"+channel), msgData).Err()
}

// PublishToNode publishes a message directly to a specific node.
func (c *Client) PublishToNode(ctx context.Context, nodeID string, msgType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msg := Message{
		Type:     msgType,
		FromNode: c.nodeID,
		Payload:  data,
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.rdb.Publish(ctx, c.key("node:"+nodeID), msgData).Err()
}

// ============================================================================
// Generic Key-Value Operations
// ============================================================================

// Set sets a key with optional TTL.
func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, c.key(key), data, ttl).Err()
}

// Get gets a value by key.
func (c *Client) Get(ctx context.Context, key string, dest any) error {
	data, err := c.rdb.Get(ctx, c.key(key)).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Delete deletes a key.
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, c.key(key)).Err()
}

// Exists checks if a key exists.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, c.key(key)).Result()
	return n > 0, err
}

// Incr increments a counter.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, c.key(key)).Result()
}

// Decr decrements a counter.
func (c *Client) Decr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Decr(ctx, c.key(key)).Result()
}

// SetNX sets a key only if it doesn't exist (for locks).
func (c *Client) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	return c.rdb.SetNX(ctx, c.key(key), data, ttl).Result()
}
