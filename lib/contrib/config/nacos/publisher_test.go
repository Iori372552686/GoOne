package nacos

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// mockConfigClient 实现 config_client.IConfigClient，仅记录 PublishConfig 入参。
type mockConfigClient struct {
	published []vo.ConfigParam
	pubOk     bool
	pubErr    error
}

func (m *mockConfigClient) PublishConfig(p vo.ConfigParam) (bool, error) {
	m.published = append(m.published, p)
	return m.pubOk, m.pubErr
}
func (m *mockConfigClient) GetConfig(_ vo.ConfigParam) (string, error) {
	return "", nil
}
func (m *mockConfigClient) DeleteConfig(_ vo.ConfigParam) (bool, error) {
	return false, nil
}
func (m *mockConfigClient) ListenConfig(_ vo.ConfigParam) error     { return nil }
func (m *mockConfigClient) CancelListenConfig(_ vo.ConfigParam) error { return nil }
func (m *mockConfigClient) SearchConfig(_ vo.SearchConfigParam) (*model.ConfigPage, error) {
	return nil, nil
}
func (m *mockConfigClient) CloseClient() {}

// 编译期断言：mockConfigClient 实现 IConfigClient。
var _ config_client.IConfigClient = (*mockConfigClient)(nil)

func TestPublisher_Publish(t *testing.T) {
	mock := &mockConfigClient{pubOk: true}
	pub, err := NewPublisher(mock, WithPubGroup("GOONE_GROUP"))
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}

	if err := pub.Publish(context.Background(), "ItemConfig.json", []byte("{\"id\":1}"), "json"); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(mock.published) != 1 {
		t.Fatalf("want 1 publish call, got %d", len(mock.published))
	}
	got := mock.published[0]
	if got.DataId != "ItemConfig.json" {
		t.Errorf("DataId=%q want ItemConfig.json", got.DataId)
	}
	if got.Group != "GOONE_GROUP" {
		t.Errorf("Group=%q want GOONE_GROUP", got.Group)
	}
	if got.Content != `{"id":1}` {
		t.Errorf("Content=%q", got.Content)
	}
	if got.Type != "json" {
		t.Errorf("Type=%q want json", got.Type)
	}
}

func TestPublisher_DefaultGroup(t *testing.T) {
	mock := &mockConfigClient{pubOk: true}
	pub, err := NewPublisher(mock)
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	if err := pub.Publish(context.Background(), "Drop.conf", []byte("a: 1"), ""); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if mock.published[0].Group != "DEFAULT_GROUP" {
		t.Errorf("default group=%q want DEFAULT_GROUP", mock.published[0].Group)
	}
	if mock.published[0].Type != "" {
		t.Errorf("empty format should not set Type, got %q", mock.published[0].Type)
	}
}

func TestPublisher_PublishFalse(t *testing.T) {
	mock := &mockConfigClient{pubOk: false, pubErr: nil}
	pub, _ := NewPublisher(mock)
	err := pub.Publish(context.Background(), "x.json", []byte("y"), "json")
	if err == nil || !strings.Contains(err.Error(), "returned false") {
		t.Fatalf("want 'returned false' error, got %v", err)
	}
}

func TestPublisher_PublishError(t *testing.T) {
	mock := &mockConfigClient{pubOk: true, pubErr: errors.New("boom")}
	pub, _ := NewPublisher(mock)
	if err := pub.Publish(context.Background(), "x.json", []byte("y"), "json"); err == nil {
		t.Fatalf("want error from PublishConfig")
	}
}

func TestPublisher_NilClient(t *testing.T) {
	if _, err := NewPublisher(nil); err == nil {
		t.Fatal("want error for nil client")
	}
}

func TestPublisher_EmptyDataID(t *testing.T) {
	mock := &mockConfigClient{pubOk: true}
	pub, _ := NewPublisher(mock)
	if err := pub.Publish(context.Background(), "  ", []byte("x"), "json"); err == nil {
		t.Fatal("want error for empty dataID")
	}
}

func TestPublisher_Close(t *testing.T) {
	pub, _ := NewPublisher(&mockConfigClient{pubOk: true})
	if err := pub.Close(); err != nil {
		t.Fatalf("Close should be no-op, got %v", err)
	}
}
