package actor

import (
	"runtime"
	"sync/atomic"

	"github.com/AsynkronIT/gam/actor/lfqueue"
)

type unboundedLockfreeMailbox struct {
	repeat          int
	throughput      int
	userMailbox     *lfqueue.LockfreeQueue
	systemMailbox   *lfqueue.LockfreeQueue
	schedulerStatus int32
	hasMoreMessages int32
	userInvoke      func(UserMessage)
	systemInvoke    func(SystemMessage)
}

func (mailbox *unboundedLockfreeMailbox) PostUserMessage(message UserMessage) {
	mailbox.userMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *unboundedLockfreeMailbox) PostSystemMessage(message SystemMessage) {
	mailbox.systemMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *unboundedLockfreeMailbox) schedule() {
	atomic.StoreInt32(&mailbox.hasMoreMessages, mailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, mailboxIdle, mailboxRunning) {
		go mailbox.processMessages()
	}
}

func (mailbox *unboundedLockfreeMailbox) Suspend() {

}

func (mailbox *unboundedLockfreeMailbox) Resume() {

}

func (mailbox *unboundedLockfreeMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&mailbox.hasMoreMessages, mailboxHasNoMessages)

	done := 0
	for done != mailbox.repeat {
		//process x messages in sequence, then exit
		for i := 0; i < mailbox.throughput; i++ {
			if sysMsg := mailbox.systemMailbox.Pop(); sysMsg != nil {
				done = 0
				sys, _ := sysMsg.(SystemMessage)
				mailbox.systemInvoke(sys)
			} else if userMsg := mailbox.userMailbox.Pop(); userMsg != nil {
				done = 0
				mailbox.userInvoke(userMsg.(UserMessage))
			} else {
				done++
				break
			}
		}
		runtime.Gosched()
	}

	//set mailbox to idle
	atomic.StoreInt32(&mailbox.schedulerStatus, mailboxIdle)
	//check if there are still messages to process (sent after the message loop ended)
	if atomic.SwapInt32(&mailbox.hasMoreMessages, mailboxHasNoMessages) == mailboxHasMoreMessages {
		mailbox.schedule()
	}

}

//NewUnboundedLockfreeMailbox creates an unbounded mailbox
func NewUnboundedLockfreeMailbox(throughput int) MailboxProducer {
	return func() Mailbox {
		userMailbox := lfqueue.NewLockfreeQueue()
		systemMailbox := lfqueue.NewLockfreeQueue()
		mailbox := unboundedLockfreeMailbox{
			repeat:          1,
			throughput:      throughput,
			userMailbox:     userMailbox,
			systemMailbox:   systemMailbox,
			hasMoreMessages: mailboxHasNoMessages,
			schedulerStatus: mailboxIdle,
		}
		return &mailbox
	}
}

func (mailbox *unboundedLockfreeMailbox) RegisterHandlers(userInvoke func(UserMessage), systemInvoke func(SystemMessage)) {
	mailbox.userInvoke = userInvoke
	mailbox.systemInvoke = systemInvoke
}
