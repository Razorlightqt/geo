# geo

Объединённые гео-базы для Xray (`geoip.dat` / `geosite.dat`), собираемые и публикуемые через GitHub Actions, раздаются через jsDelivr.

## Зачем

Клиентское ядро Xray читает **ровно один** источник `geosite.dat`/`geoip.dat` (гео — только файлы с диска). Поэтому совместить полную каноническую базу с РКН-кастомом roscomvpn на клиенте можно только в **одном** объединённом файле. Этот репозиторий его и собирает.

## Состав (два namespace, без коллизий)

- **Канонические категории** из [`v2fly/domain-list-community`](https://github.com/v2fly/domain-list-community) (geosite) и общей geoip-базы — под оригинальными именами: `geosite:google`, `geoip:ru`, `geosite:private`, …
- **roscomvpn поверх с префиксом** `roscomvpn-` ([`hydraponique/roscomvpn-geosite`](https://github.com/hydraponique/roscomvpn-geosite) + [`-geoip`](https://github.com/hydraponique/roscomvpn-geoip)): `geosite:roscomvpn-category-ru`, `geoip:roscomvpn-whitelist`, …

Одноимённые категории не сливаются — сосуществуют, обе версии доступны для маршрутизации.

## URL (jsDelivr)

```
https://cdn.jsdelivr.net/gh/Razorlightqt/geo@release/geosite.dat
https://cdn.jsdelivr.net/gh/Razorlightqt/geo@release/geoip.dat
```

Ветка `release` обновляется при каждой сборке (плавающий URL, контент меняется), плюс публикуется версионный GitHub Release (тег `YYYYMMDDHHMM`).

## Сборка

GitHub Actions, ежедневно по расписанию + ручной запуск (`workflow_dispatch`). Серверы проекта в сборке не участвуют.

- **geosite**: полный `data/` v2fly + 23 категории roscomvpn, переименованные в `roscomvpn-*`, компилируются компилятором `domain-list-community` одним прогоном.
- **geoip**: общая база + кастом roscomvpn, склеенные на уровне protobuf с префиксом `roscomvpn-` у записей roscomvpn.

## Атрибуция / лицензии

Сборка из публичных upstream-источников: `v2fly/domain-list-community` (MIT), `hydraponique/roscomvpn-geosite` / `roscomvpn-geoip`, `Loyalsoldier/geoip` (общая geoip-база). Права на исходные данные принадлежат их авторам.
