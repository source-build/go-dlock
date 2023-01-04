package go_dlock

import (
	"errors"
	"net"
	"sync"
	"time"
)

type DLock struct {
	address    string
	secretKey  string
	conn       net.Conn
	isActive   bool
	isLock     bool
	unlockChan chan error
	await      chan struct{}
}

func NewDLock(remoteAddr, secretKey string) *DLock {
	return &DLock{address: remoteAddr, secretKey: secretKey}
}

func (l *DLock) Lock(key string) error {
	if l.isLock {
		return errors.New("do not repeat the operation")
	}
	if !l.isActive {
		conn, err := net.Dial("tcp", l.address)
		if err != nil {
			return err
		}
		l.conn = conn
		l.isActive = true
	}
	if err := l.authentication(); err != nil {
		return err
	}

	l.isLock = true
	l.await = make(chan struct{}, 1)
	go l.listen()
	return l.lockProcess(key)
}

func (l *DLock) UnLock() error {
	if !l.isActive {
		return errors.New("the lock has not been applied yet, but an attempt has been made to unlock it")
	}
	defer func() { l.isLock = false }()
	if err := l.write(UnLockEvent); err != nil {
		return err
	}

	val := <-l.unlockChan
	return val
}

func (l *DLock) lockProcess(key string) error {
	if err := l.write(LockEvent, []byte(key)); err != nil {
		return err
	}
	buf := make([]byte, 2)
	err := l.conn.SetReadDeadline(time.Now().Add(time.Second * 60))
	if err != nil {
		return err
	}
	n, err := l.conn.Read(buf)
	if err != nil {
		return err
	}

	event, _, err := decode(buf[:n])
	if err != nil {
		return err
	}

	l.await <- struct{}{}
	if event == LockOKEvent {
		return nil
	}
	switch event {
	case OperateTimeoutEvent:
		return errors.New("not unlocked in time")
	case LockFailEvent:
		return errors.New("congestion ahead")
	case alreadyLockEvent:
		return errors.New("do not repeat the operation")
	case notFindLockEvent:
		return errors.New("the lock has not been applied yet, but an attempt has been made to unlock it")
	}
	return errors.New("busy")
}

func (l *DLock) authentication() error {
	if err := l.write(authEvent, []byte(l.secretKey)); err != nil {
		return err
	}
	buf := make([]byte, 256)
	//if err := l.conn.SetReadDeadline(time.Now().Add(time.Second * 10)); err != nil {
	//	return err
	//}
	n, err := l.conn.Read(buf)
	if err != nil {
		return err
	}
	event, _, err := decode(buf[:n])
	if err != nil {
		return err
	}
	if event != authOKEvent {
		return errors.New("identity authentication failed")
	}
	return nil
}

func (l *DLock) listen() {
	<-l.await
	l.unlockChan = make(chan error, 1)
	buf := make([]byte, 2)
	defer l.close()
	for {
		n, err := l.conn.Read(buf)
		if err != nil {
			return
		}
		event, _, err := decode(buf[:n])
		if err != nil {
			return
		}
		if event == LockOKEvent {
			continue
		}
		if event == UnLockOKEvent {
			l.unlockChan <- nil
			return
		}
		if event == notFindLockEvent {
			l.unlockChan <- errors.New("the lock has not been applied yet, but an attempt has been made to unlock it")
			return
		}
		if event == OperateTimeoutEvent {
			l.unlockChan <- errors.New("not unlocked in time")
		}
		l.isLock = false
		return
	}
}

func (l *DLock) write(event eventType, message ...[]byte) error {
	body, err := encode(event, message...)
	if err != nil {
		return errors.New("encode fail")
	}
	_, err = l.conn.Write(body)
	if err != nil {
		return err
	}
	return nil
}

func (l *DLock) close() {
	l.isActive = false
	l.isLock = false
	l.closeConn()
}

func (l *DLock) closeConn() {
	new(sync.Once).Do(func() {
		_ = l.conn.Close()
	})
}
