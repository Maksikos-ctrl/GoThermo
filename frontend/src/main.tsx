import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
// Last on purpose: the palettes must come after every other stylesheet.
import './styles/palettes.generated.css'

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <App/>
    </React.StrictMode>
)
