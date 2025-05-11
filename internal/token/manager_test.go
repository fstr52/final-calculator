package token

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const testSigningKey = "test_secret_key"

func TestNewManager(t *testing.T) {
	m, err := NewManager(testSigningKey)
	assert.NoError(t, err)
	assert.NotNil(t, m)

	m, err = NewManager("")
	assert.Error(t, err)
	assert.Nil(t, m)
}

func TestManager_NewToken_And_Parse(t *testing.T) {
	m, err := NewManager(testSigningKey)
	assert.NoError(t, err)

	userId := "user123"
	ttl := time.Minute

	tokenStr, err := m.NewToken(userId, ttl)
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	parsedUserId, err := m.Parse(tokenStr)
	assert.NoError(t, err)
	assert.Equal(t, userId, parsedUserId)
}

func TestManager_Parse_InvalidToken(t *testing.T) {
	m, _ := NewManager(testSigningKey)

	// Подделанный токен
	invalidToken := "invalid.token.string"
	_, err := m.Parse(invalidToken)
	assert.Error(t, err)

	// Токен с другим ключом
	otherManager, _ := NewManager("other_key")
	tokenStr, _ := otherManager.NewToken("user", time.Minute)
	_, err = m.Parse(tokenStr)
	assert.Error(t, err)
}

func TestManager_NewRefreshToken(t *testing.T) {
	m, _ := NewManager(testSigningKey)
	token1, err := m.NewRefreshToken()
	assert.NoError(t, err)
	assert.Len(t, token1, 64)

	time.Sleep(1 * time.Second)

	token2, err := m.NewRefreshToken()
	assert.NoError(t, err)
	assert.NotEqual(t, token1, token2) // Теперь всегда будет разным
}
