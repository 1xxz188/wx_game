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

package component

// Component is the interface that represent a component.
type Component interface {
	Name() string
	Init() error
	AfterInit() //只能成功 --热更组件调用
	BeforeShutdown()
	Shutdown()
}
