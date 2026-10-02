'use strict'
// Bean playground compile/render worker. Loads the Go-compiled bean.wasm next
// to this script and answers {id, request} messages with {id, response}
// where request/response are bridge protocol v1 JSON documents.

let wasmReady = false

function respond(id, response) {
  self.postMessage({type: 'result', id: id, response: response})
}

self.onmessage = function (event) {
  const message = event.data
  if (!message || typeof message.id !== 'number') return
  if (!wasmReady || typeof self.beanPlayground !== 'function') {
    respond(message.id, {v: 1, ok: false, error: {code: 'BEAN-P4100', message: 'the compiler is still loading'}})
    return
  }
  try {
    respond(message.id, JSON.parse(self.beanPlayground(JSON.stringify(message.request))))
  } catch (error) {
    respond(message.id, {v: 1, ok: false, error: {code: 'BEAN-P4205', message: 'the compiler crashed: ' + String((error && error.message) || error)}})
  }
}

try {
  importScripts(new URL('wasm_exec.js', self.location.href).href)
  const go = new Go()
  fetch(new URL('bean.wasm', self.location.href))
    .then(function (response) {
      if (!response.ok) throw new Error('bean.wasm fetch failed with HTTP ' + response.status)
      return response.arrayBuffer()
    })
    .then(function (bytes) { return WebAssembly.instantiate(bytes, go.importObject) })
    .then(function (result) {
      // go.run resolves only when the program exits; beanwasm blocks forever.
      go.run(result.instance).catch(function (error) {
        wasmReady = false
        self.postMessage({type: 'fatal', error: 'the compiler exited: ' + String((error && error.message) || error)})
      })
      const poll = setInterval(function () {
        if (typeof self.beanPlayground === 'function') {
          clearInterval(poll)
          wasmReady = true
          self.postMessage({type: 'ready'})
        }
      }, 25)
      setTimeout(function () {
        if (!wasmReady) {
          clearInterval(poll)
          self.postMessage({type: 'fatal', error: 'the compiler did not finish starting'})
        }
      }, 30000)
    })
    .catch(function (error) {
      self.postMessage({type: 'fatal', error: 'bean.wasm failed to load: ' + String((error && error.message) || error)})
    })
} catch (error) {
  self.postMessage({type: 'fatal', error: 'wasm_exec.js failed to load: ' + String((error && error.message) || error)})
}
