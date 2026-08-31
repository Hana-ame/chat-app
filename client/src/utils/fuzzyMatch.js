// fuzzyMatch.js — 模糊匹配评分（【本地改动 2026-09-03】Cmd-K 快速切换，对齐 FDR-015）
//
// 简单子序列匹配：query 的字符按顺序在 target 中出现即算匹配（fuzzy）。
// 连续匹配得分更高（pow 放大），label 匹配加权。返回 {score, matches}。
// 目的：ChatList 现有搜索是「includes」，Cmd-K 需要子序列模糊，让
// "ch" 能命中 "chats"、"gch" 能命中 "group chat general" 这类非连续输入。

/**
 * @param {string} query
 * @param {string} target
 * @returns {{score:number, matches:number[]} | null}
 */
export function fuzzyMatch(query, target) {
  if (!query) return null;
  const q = query.toLowerCase();
  const t = target.toLowerCase();
  let i = 0;
  let lastMatch = -1;
  let run = 0;
  let score = 0;
  const matches = [];
  for (let j = 0; j < t.length && i < q.length; j++) {
    if (t[j] === q[i]) {
      matches.push(j);
      run = j === lastMatch + 1 ? run + 1 : 1;
      score += 1 + run * run; // 连续匹配高权重
      lastMatch = j;
      i++;
    }
  }
  if (i < q.length) return null; // 未能按序覆盖 query
  return { score, matches };
}

// rankResults 组合 label/detail 两个域的分数，label 权重更高。
// detail 通常放聊天名（room 名）、成员名等次要信息。
/** @returns {number} */
export function combinedScore(labelScore, detailScore, labelWeight = 3) {
  return (labelScore || 0) * labelWeight + (detailScore || 0);
}
