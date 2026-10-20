# highload_prog

Для запуска нужны Docker и Docker Compose.

Создайте файл `.env` в корне проекта. Вместо `<пароль_PostgreSQL>` укажите пароль базы данных:

```dotenv
POSTGRES_PASSWORD=<пароль_PostgreSQL>
GRAFANA_ADMIN_PASSWORD=password
```

Из корня проекта выполните:

```bash
docker compose up --build
```

Откройте Grafana: [http://localhost:3000](http://localhost:3000).

- Логин: `admin`.
- Пароль: `password`.

Дашборд **Highload HTTP** создаётся автоматически. После его создания запускается фоновая нагрузка k6.
