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

type (
	options struct {
		name string // component name
	}

	// Option used to customize handler
	Option func(options *options)
)

// WithName used to rename component name
func WithName(name string) Option {
	return func(opt *options) {
		opt.name = name
	}
}
