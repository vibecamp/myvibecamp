# my.vibecamp.xyz

## Build

I develop and deploy on Ubuntu Linux, so no guarantees about other OSes.

Requires Go for development and Docker for deployment. I'll asume you have those.

```
cd myvibecamp
go get
cp env.example env
# fill in env with your credentials
./dev.sh
```

### Twitter API Access

- https://developer.twitter.com/en/apply-for-access: apply for a developer account (may take several days to be approved)
- https://developer.twitter.com/en/portal/projects-and-apps: create a new app and get tokens
- In the app settings page under `User authentication settings`, click `Set up`
- Turn on `OAuth 1.0a`
- Leave `oauth 1.0a settings` as is (`Request email from users`: `disabled`, `App permissions`: `Read`)
- Set `Callback URI / Redirect URL` to the value of `EXTERNAL_URL` with `/callback` appended (by default `http://127.0.0.1.nip.io:8080/callback`)
- Set `Website URL` to the value of `EXTERNAL_URL` (by default `http://127.0.0.1.nip.io:8080`)

### Airtable API Access

- https://airtable.com/account: access your API key
- https://airtable.com/api
- https://support.airtable.com/hc/en-us/articles/4405741487383-Understanding-Airtable-IDs

### Airtable Table Setup

- Create a new table by importing [empty-table.csv](empty-table.csv)
- Use the first part of the URL (starts with `app`) as your `AIRTABLE_BASE_ID`
- Use the second part of the URL (starts with `tbl`) as your `AIRTABLE_TABLE_NAME`
- For each user you want to test with, you need to add a new row. It must at least have:
  - Their twitter handle (without the `@`) in the `twitter clean` column
  - Some text in the `Barcode` column

## Deploy

```
./dockerpush.sh
```

Pushing to Docker Hub will automatically deploy the image to production.
