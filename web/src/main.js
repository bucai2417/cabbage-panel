import { createApp } from 'vue'
import App from './App.vue'
import { installCardTilt } from './tilt'
import './style.css'
import './content.css'
import './panels.css'
import './responsive.css'
import './workspace.css'
import './ui.css'
import './apidocs.css'

createApp(App).mount('#app')
installCardTilt()
