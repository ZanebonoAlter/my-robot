import { describe, expect, it } from 'vitest'
import { buildRouteParamSpecs, parseParameterDescriptions, parsePathParams } from './routeParams'

describe('parsePathParams', () => {
  it('returns empty for path without params', () => {
    expect(parsePathParams('/81rc/news')).toEqual([])
  })

  it('marks bare :param as required', () => {
    expect(parsePathParams('/bilibili/user/video/:uid')).toEqual([
      { name: 'uid', required: true },
    ])
  })

  it('marks :param? as optional', () => {
    expect(parsePathParams('/81rc/:category?')).toEqual([
      { name: 'category', required: false },
    ])
  })

  it('strips regex constraint and treats :param{.+}? as optional', () => {
    expect(parsePathParams('/81rc/:category{.+}?')).toEqual([
      { name: 'category', required: false },
    ])
  })

  it('strips regex constraint on required param', () => {
    expect(parsePathParams('/weibo/user/:uid{[0-9]+}')).toEqual([
      { name: 'uid', required: true },
    ])
  })

  it('parses multiple params preserving order', () => {
    expect(parsePathParams('/zhihu/question/:questionId/:sort?')).toEqual([
      { name: 'questionId', required: true },
      { name: 'sort', required: false },
    ])
  })
})

describe('parseParameterDescriptions', () => {
  it('parses object with plain string descriptions', () => {
    expect(parseParameterDescriptions('{"category":"分类，默认为全部"}')).toEqual({
      category: '分类，默认为全部',
    })
  })

  it('parses object with nested description field', () => {
    expect(parseParameterDescriptions('{"uid":{"description":"用户 id"}}')).toEqual({
      uid: '用户 id',
    })
  })

  it('parses string array form', () => {
    expect(parseParameterDescriptions('["uid","sort"]')).toEqual({ uid: '', sort: '' })
  })

  it('parses array of objects form', () => {
    expect(parseParameterDescriptions('[{"name":"uid","description":"用户 id"}]')).toEqual({
      uid: '用户 id',
    })
  })

  it('returns empty object for invalid JSON', () => {
    expect(parseParameterDescriptions('not-json')).toEqual({})
  })

  it('returns empty object for empty input', () => {
    expect(parseParameterDescriptions('')).toEqual({})
    expect(parseParameterDescriptions('{}')).toEqual({})
  })
})

describe('buildRouteParamSpecs', () => {
  it('merges path params with descriptions, required first', () => {
    const specs = buildRouteParamSpecs(
      '/zhihu/question/:questionId/:sort?',
      '{"questionId":"问题 id","sort":"排序方式"}',
    )
    expect(specs).toEqual([
      { name: 'questionId', required: true, description: '问题 id' },
      { name: 'sort', required: false, description: '排序方式' },
    ])
  })

  it('falls back to empty description when catalog has no entry', () => {
    const specs = buildRouteParamSpecs('/bilibili/user/video/:uid', '{}')
    expect(specs).toEqual([{ name: 'uid', required: true, description: '' }])
  })
})
