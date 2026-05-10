// Bangumi 图片地址工具
// URL 格式:
//   封面: https://lain.bgm.tv/pic/cover/s|c|l/xx.jpg
//   角色/人物: https://lain.bgm.tv/pic/crt/s|m|l/xx.jpg  或 /pic/crt/g/ (grid)
//   resize:  https://lain.bgm.tv/r/400/pic/cover/l/xx.jpg

/**
 * 规范化 Bangumi 图片 URL：
 *  - http -> https（避免 mixed content）
 *  - 封面/角色 小图/中图/grid -> 大图
 */
function normalizeBangumi(url) {
  if (!url) return url
  let u = url.replace(/^http:\/\//, 'https://')
  // 封面：s/c -> l
  u = u.replace(/\/pic\/cover\/[sc]\//, '/pic/cover/l/')
  // 角色：g(grid)/s/m -> l
  u = u.replace(/\/pic\/crt\/[gsm]\//, '/pic/crt/l/')
  // 通用：其他路径的 /s/ /m/ 也尝试替换（小图 URL 模式）
  return u
}

/**
 * 返回高清图（原图，不 resize）
 */
export function toHighResImage(url) {
  if (!url) return url
  return normalizeBangumi(url).replace(/\/r\/\d+\//, '/')
}

/**
 * 返回指定宽度的图
 */
export function toResizedImage(url, width = 800) {
  if (!url) return url
  const u = normalizeBangumi(url)
  if (/\/r\/\d+\//.test(u)) {
    return u.replace(/\/r\/\d+\//, `/r/${width}/`)
  }
  return u.replace(/(lain\.bgm\.tv)\//, `$1/r/${width}/`)
}
