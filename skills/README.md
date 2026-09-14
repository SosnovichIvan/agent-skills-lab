# Skills

Здесь размещаются навыки, которые расширяют и оптимизируют работу агентов.

`SKILL.md` является ядром навыка. В модульных skills обязательные материалы
хранятся в `references/common/`, а выбираемые языковые и framework-профили — в
`references/options/`. Для frontend skill установка требует одной структуры:
FSD, flexible logic-based или собственного UTF-8 файла. У полностью связанного
skill references могут оставаться
непосредственно в `references/` и устанавливаться комплектом. Связь профиля с
родительским skill описана в [`catalog.json`](catalog.json); отдельно reference
не устанавливается.

| Название файла | Ссылка на файл | Название скила или правила | Описание: для чего нужно и что делает |
| --- | --- | --- | --- |
| `skill-authoring/SKILL.md` | [`SKILL.md`](skill-authoring/SKILL.md) | Оформление навыков | Устанавливает структуру папок, правила для `SKILL.md`, добавления references и ведения каталога навыков. |
| `execution-state/SKILL.md` | [`SKILL.md`](execution-state/SKILL.md) | Выполнение через состояние | Версия `1.0.0`: универсальный координатор для любого agent CLI с explicit/project-policy активацией, semantic chunks, compact state/context map, quality gates и adaptive passthrough/lite/reset handoff. |
| `ui-ux-design/SKILL.md` | [`SKILL.md`](ui-ux-design/SKILL.md) | UI/UX design | Независимый skill с локальным поиском visual direction, design-system, accessibility, interaction, responsive, typography, color и chart-рекомендаций; устанавливается целиком и не требует других skills. |
| `frontend-engineering/SKILL.md` | [`SKILL.md`](frontend-engineering/SKILL.md) | Frontend engineering | Ядро frontend skill с очищенными соглашениями именования; установщик дополняет его language/framework references и обязательной структурой FSD, flexible logic-based или пользовательскими правилами. |
| `backend-engineering/SKILL.md` | [`SKILL.md`](backend-engineering/SKILL.md) | Backend engineering | Ядро backend skill; установщик дополняет его только выбранными references для Go, Node.js/TypeScript, Python, Spring Boot, ASP.NET Core или Laravel. |
| `frontend-review/SKILL.md` | [SKILL.md](frontend-review/SKILL.md) | Frontend Review | Ревью frontend-кода с доказательствами функциональных регрессий. |
| `backend-review/SKILL.md` | [SKILL.md](backend-review/SKILL.md) | Backend Review | Ревью контрактов, авторизации, конкурентности и отказов сервисов. |
| `database-engineering/SKILL.md` | [SKILL.md](database-engineering/SKILL.md) | Database Engineering | Схемы, SQL, индексы, миграции и backfill реляционных БД. |
| `database-review/SKILL.md` | [SKILL.md](database-review/SKILL.md) | Database Review | Ревью SQL, миграций, изоляции данных и совместимости rollout. |
| `project-architecture/SKILL.md` | [SKILL.md](project-architecture/SKILL.md) | Project Architecture | Границы системы, контракты, ADR и план реализации/перехода. |
| `content-writing/SKILL.md` | [SKILL.md](content-writing/SKILL.md) | Content Writing | Создание и редактура продуктовых, маркетинговых и технических текстов. |
| `ux-writing/SKILL.md` | [SKILL.md](ux-writing/SKILL.md) | UX Writing | Тексты действий, форм, ошибок и состояний интерфейса. |
| `brand-logo-design/SKILL.md` | [SKILL.md](brand-logo-design/SKILL.md) | Brand Logo Design | Концепция логотипа, векторный мастер и варианты использования. |
| `icon-system/SKILL.md` | [SKILL.md](icon-system/SKILL.md) | Icon System | Подбор, создание и интеграция согласованного набора SVG-иконок. |
