package 管理接口

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

const (
	会话有效期    = 12 * time.Hour
	会话Cookie = "asterwaf_session"
)

type 会话值 struct {
	U用户名 string
	U到期  time.Time
}

// U会话管理 只保存内存会话；重启即全体登出是刻意的取舍。
type U会话管理 struct {
	互斥   sync.Mutex
	会话表  map[string]会话值
	时钟   func() time.Time
	停止   chan struct{}
	完成   chan struct{}
	关闭一次 sync.Once
}

func U新建会话() *U会话管理 {
	return 新建会话(time.Now, 10*time.Minute)
}

func 新建会话(时钟 func() time.Time, 清理间隔 time.Duration) *U会话管理 {
	管理 := &U会话管理{
		会话表: make(map[string]会话值),
		时钟:  时钟,
		停止:  make(chan struct{}),
		完成:  make(chan struct{}),
	}
	if 清理间隔 <= 0 {
		close(管理.完成)
		return 管理
	}
	go 管理.清理循环(清理间隔)
	return 管理
}

func (管理 *U会话管理) U创建(用户名 string) (string, time.Time, error) {
	var 随机 [32]byte
	if _, 错误 := rand.Read(随机[:]); 错误 != nil {
		return "", time.Time{}, 错误
	}
	令牌 := base64.RawURLEncoding.EncodeToString(随机[:])
	现在 := 管理.时钟()
	到期 := 现在.Add(会话有效期)
	管理.互斥.Lock()
	管理.会话表[令牌] = 会话值{U用户名: 用户名, U到期: 到期}
	管理.互斥.Unlock()
	return 令牌, 到期, nil
}

func (管理 *U会话管理) U获取(令牌 string) (会话值, bool) {
	现在 := 管理.时钟()
	管理.互斥.Lock()
	defer 管理.互斥.Unlock()
	会话, 存在 := 管理.会话表[令牌]
	if !存在 {
		return 会话值{}, false
	}
	if !现在.Before(会话.U到期) {
		delete(管理.会话表, 令牌)
		return 会话值{}, false
	}
	return 会话, true
}

func (管理 *U会话管理) U删除(令牌 string) {
	管理.互斥.Lock()
	delete(管理.会话表, 令牌)
	管理.互斥.Unlock()
}

func (管理 *U会话管理) Close() {
	管理.关闭一次.Do(func() {
		close(管理.停止)
	})
	<-管理.完成
}

func (管理 *U会话管理) 清理循环(间隔 time.Duration) {
	defer close(管理.完成)
	计时器 := time.NewTicker(间隔)
	defer 计时器.Stop()
	for {
		select {
		case <-计时器.C:
			管理.清理(管理.时钟())
		case <-管理.停止:
			return
		}
	}
}

func (管理 *U会话管理) 清理(现在 time.Time) {
	管理.互斥.Lock()
	defer 管理.互斥.Unlock()
	for 令牌, 会话 := range 管理.会话表 {
		if !现在.Before(会话.U到期) {
			delete(管理.会话表, 令牌)
		}
	}
}
