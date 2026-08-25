import { describe, expect, it } from 'vitest'
import { isValidKeywordExpr, parseKeywordExpr, parseKeywordSlots } from './keywordExpr'

// 对齐后端 test-cases.md 白盒 B3-B7 / 变体表 V1-V6 + 冻结语法契约
// （design.md §4.2：空格=AND、| = OR、`ASML|镓锗 出口` = (ASML 或 镓锗) 且含 出口）。
describe('keywordExpr — parseKeywordExpr（OR-of-ANDs 笛卡尔展开）', () => {
  it('B4 单词：ASML → [[ASML]]', () => {
    expect(parseKeywordExpr('ASML')).toEqual([['ASML']])
  })

  it('B5 空格 AND：出口 限制 → [[出口,限制]]', () => {
    expect(parseKeywordExpr('出口 限制')).toEqual([['出口', '限制']])
  })

  it('B6 | OR：ASML|镓锗 → [[ASML],[镓锗]]', () => {
    expect(parseKeywordExpr('ASML|镓锗')).toEqual([['ASML'], ['镓锗']])
  })

  it('B7 混用：ASML|镓锗 出口 → [[ASML,出口],[镓锗,出口]]（= (ASML 或 镓锗) 且含 出口）', () => {
    expect(parseKeywordExpr('ASML|镓锗 出口')).toEqual([['ASML', '出口'], ['镓锗', '出口']])
  })

  it('B1 空串 → []', () => {
    expect(parseKeywordExpr('')).toEqual([])
  })

  it('B2/V2 纯空白（全角空格 + tab + 半角混合）→ []', () => {
    expect(parseKeywordExpr('　\t ')).toEqual([])
    expect(parseKeywordExpr('   ')).toEqual([])
    expect(parseKeywordExpr('\t')).toEqual([])
  })

  it('B3/V3 纯分隔符：| / || / | | → []', () => {
    expect(parseKeywordExpr('|')).toEqual([])
    expect(parseKeywordExpr('||')).toEqual([])
    expect(parseKeywordExpr('| |')).toEqual([])
  })

  it('B3 槽内空洞：ASML|（尾随 |）→ []（无效）', () => {
    expect(parseKeywordExpr('ASML|')).toEqual([])
  })

  it('B3 前导独立 | token：| 出口 → [[出口]]（容忍丢弃冗余分隔符）', () => {
    expect(parseKeywordExpr('| 出口')).toEqual([['出口']])
  })

  it('B3 前导 | 紧贴词可容忍，尾随 | 仍无效', () => {
    expect(parseKeywordExpr('|出口')).toEqual([['出口']])
    expect(parseKeywordExpr('出口|')).toEqual([])
  })

  it('连续分隔符混词：A||B 忽略空分支 → [[A],[B]]', () => {
    expect(parseKeywordExpr('A||B')).toEqual([['A'], ['B']])
  })

  it('V6 正则元字符按字面处理：C++ / .* 不报错、按字面成词', () => {
    expect(parseKeywordExpr('C++')).toEqual([['C++']])
    expect(parseKeywordExpr('.*')).toEqual([['.*']])
  })

  it('V8 emoji / 引号按字面成词（AND 槽合成单分支）', () => {
    // 两个 emoji 词 = AND 槽 → 笛卡尔展开为单分支（含两词），非 OR
    expect(parseKeywordExpr('🇺🇸 制裁')).toEqual([['🇺🇸', '制裁']])
    // 预览侧：两槽各自独立
    expect(parseKeywordSlots('🇺🇸 制裁')).toEqual([['🇺🇸'], ['制裁']])
  })

  it('多备选多槽混合笛卡尔：A|B C|D → 4 分支', () => {
    expect(parseKeywordExpr('A|B C|D')).toEqual([
      ['A', 'C'],
      ['A', 'D'],
      ['B', 'C'],
      ['B', 'D'],
    ])
  })

  it('词形保留（大小写由匹配侧处理，解析不折叠）：asml → [[asml]]', () => {
    expect(parseKeywordExpr('asml')).toEqual([['asml']])
  })
})

describe('keywordExpr — parseKeywordSlots（解析预览 chips）', () => {
  it('ASML|镓锗 出口 → [[ASML,镓锗],[出口]]（展示 [ASML|镓锗] × [出口]）', () => {
    expect(parseKeywordSlots('ASML|镓锗 出口')).toEqual([['ASML', '镓锗'], ['出口']])
  })

  it('单词 / 多词 AND / 多词 OR', () => {
    expect(parseKeywordSlots('ASML')).toEqual([['ASML']])
    expect(parseKeywordSlots('出口 限制')).toEqual([['出口'], ['限制']])
    expect(parseKeywordSlots('ASML|镓锗')).toEqual([['ASML', '镓锗']])
  })

  it('空 / 无效 → []（对话框据此显示红字提示）', () => {
    expect(parseKeywordSlots('')).toEqual([])
    expect(parseKeywordSlots('　 ')).toEqual([])
    expect(parseKeywordSlots('| |')).toEqual([])
    expect(parseKeywordSlots('ASML|')).toEqual([])
  })
})

describe('keywordExpr — isValidKeywordExpr（提交门禁）', () => {
  it('有效表达式', () => {
    expect(isValidKeywordExpr('ASML')).toBe(true)
    expect(isValidKeywordExpr('ASML|镓锗 出口')).toBe(true)
    expect(isValidKeywordExpr('| 出口')).toBe(true)
    expect(isValidKeywordExpr('|出口')).toBe(true)
    expect(isValidKeywordExpr('A||B')).toBe(true)
  })

  it('无效表达式（后端将 400，前端提前禁提交）', () => {
    expect(isValidKeywordExpr('')).toBe(false)
    expect(isValidKeywordExpr('　\t')).toBe(false)
    expect(isValidKeywordExpr('|')).toBe(false)
    expect(isValidKeywordExpr('||')).toBe(false)
    expect(isValidKeywordExpr('ASML|')).toBe(false)
  })
})
