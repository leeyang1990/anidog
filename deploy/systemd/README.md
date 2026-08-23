# AniDog 定时备份

默认每天 03:30 备份 PostgreSQL、qBittorrent 配置和 `.env`，文件权限为 `600`，保留 14 天。
媒体文件不在此任务中重复备份。

```bash
sudo install -m 644 deploy/systemd/anidog-backup.service /etc/systemd/system/
sudo install -m 644 deploy/systemd/anidog-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now anidog-backup.timer
```

部署目录不是 `/root/anidog` 时，先修改 service 中的 `WorkingDirectory` 和 `ExecStart`。
可通过 `ANIDOG_BACKUP_DIR` 和 `ANIDOG_BACKUP_RETENTION_DAYS` 覆盖备份目录及保留天数。
