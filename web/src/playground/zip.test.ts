// @vitest-environment node
// jsdom's Blob has no arrayBuffer(); Node's does, and the zip helpers only
// need standard Blob/DecompressionStream/TextCodec globals.
import {describe,expect,it} from 'vitest'
import {readZip,writeZip} from './zip'

describe('zip archive',()=>{
  it('round-trips a multi-file sources archive',async()=>{
    const files={'app.yaml':'apiVersion: bean/v1alpha1\nname: Test\n','blocks/one.yaml':'kind: Block\nname: one\n'}
    const blob=await writeZip(files)
    expect(blob.type).toBe('application/zip')
    const restored=await readZip(blob)
    expect(Object.keys(restored).sort()).toEqual(['app.yaml','blocks/one.yaml'])
    expect(restored['app.yaml']).toBe(files['app.yaml'])
    expect(restored['blocks/one.yaml']).toBe(files['blocks/one.yaml'])
  })

  it('reads a deflated zip entry',async()=>{
    // Minimal zip with one deflate-compressed entry produced by python's
    // zipfile module (compression=ZIP_DEFLATED).
    const bytes=Uint8Array.from(atob(DEFLATED_ZIP_B64),c=>c.charCodeAt(0))
    const files=await readZip(new Blob([bytes]))
    expect(files['hello.yaml']).toContain('kind: Block')
  })
})

const DEFLATED_ZIP_B64='UEsDBBQAAAAIAG50Ql0nAk9kGgAAABgAAAAKAAAAaGVsbG8ueWFtbMvOzEuxUnDKyU/O5spLzE21UshIzcnJ5wIAUEsBAhQDFAAAAAgAbnRCXScCT2QaAAAAGAAAAAoAAAAAAAAAAAAAAIABAAAAAGhlbGxvLnlhbWxQSwUGAAAAAAEAAQA4AAAAQgAAAAAA'
