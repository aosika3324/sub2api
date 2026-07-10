import { mergeLocale } from '../mergeLocale'
import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import admin from './admin'
import misc from './misc'
import fork from './fork'

const base = {
  ...landing,
  ...common,
  ...dashboard,
  admin,
  ...misc,
}

// Layer fork-specific keys (imageStudio, videoStudio, home, admin/nav additions)
// over the upstream base. Deep-merge so fork additions to shared namespaces
// (e.g. admin.*) don't clobber upstream siblings.
export default mergeLocale(base, fork)
