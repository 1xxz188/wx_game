/*
 * Copyright © 2023  IGG & The Authors Team. All rights reserved.
 *
 * This software and associated documentation files (the "Software"),
 * are proprietary to IGG & The Authors Team and are not to be copied, reproduced, or transmitted in any form,
 * in whole or in part, without the express written consent of IGG or The Authors Team.
 *
 * No part of the Software, including this file, may be copied, modified, propagated,
 * or distributed except according to the terms contained in the License Agreement.
 *
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
 * WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package main

/*
type MsgHandler interface {
	GetMsgIDs() map[msg.ID]struct{}
}

type BaseHandler struct {
	handleServer *HandleServer
	msgRPCHandle map[msg.ID]struct{}
}

func NewBaseHandler(handleServer *HandleServer) BaseHandler {
	return BaseHandler{
		msgRPCHandle: make(map[msg.ID]struct{}),
		handleServer: handleServer,
	}
}

func (bh *BaseHandler) RegisterMsgId(msgId msg.ID, fn fw.TypeHandle) error {
	if _, ok := bh.msgRPCHandle[msgId]; ok {
		return fmt.Errorf("registerMsgId[%d] repeated", msgId)
	}

	if err := bh.handleServer.RegisterMsgId(msgId, fn); err != nil {
		return err
	}

	bh.msgRPCHandle[msgId] = struct{}{}
	return nil
}

func (bh *BaseHandler) RegisterMsgIdByReplace(msgId msg.ID, fn fw.TypeHandle) error {
	err := bh.handleServer.RegisterMsgIdByReplace(msgId, fn)
	if err != nil {
		return err
	}
	bh.msgRPCHandle[msgId] = struct{}{}
	return nil
}

func (bh *BaseHandler) GetMsgIDs() map[msg.ID]struct{} {
	return bh.msgRPCHandle
}

func AddComponent(c component.Component) error {
	if app.state.Load() != AppStatusNone {
		return errors.New("register a component before app init")
	}
	if c.Name() == "" {
		return errors.New("register a component without name")
	}
	if c == nil {
		return errors.New("register a nil component")
	}

	logger.Log.Info("register component : ", c.Name())
	app.muComp.Lock()
	defer app.muComp.Unlock()

	if _, ok := app.registeredComp[c.Name()]; ok {
		return errors.New("component is already registered : " + c.Name())
	} else {
		app.registeredComp[c.Name()] = c
	}
	app.handlerComp = append(app.handlerComp, c) //component.GetServiceName(c, options)
	return nil
}

// GetComponent 如果在组件的Init、AfterInit中调用 则不能加锁 (否则热更会导致死锁)
func GetComponent(name string, needLock bool) (component.Component, error) {
	if needLock {
		app.muComp.Lock()
		defer app.muComp.Unlock()
	}

	if v, ok := app.registeredComp[name]; ok {
		return v, nil
	}
	return nil, errors.New("component not found : " + name)
}
*/
