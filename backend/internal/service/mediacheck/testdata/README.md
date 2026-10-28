# 硬字幕回归片段

两张仅保留字幕区域的裁图来自 2026-10-28 失败成片复核，分别覆盖繁体模型漏读简体、简体模型归一化繁体的情况。它们不提供剧集身份或入库授权。

`ANIDOG_OCR_FIXTURE=1 go test ./internal/service/mediacheck -run TestOutlinedCaptionOCRFixtures` 使用正式运行环境的 tesseract、chi_sim 和 chi_tra；普通测试另验证预算、读数分歧、取消与双语行隔离。不得为了特定模型版本降低置信度或简繁判定门槛。
