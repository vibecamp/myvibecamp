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



### Airtable API Access

- https://airtable.com/api
- https://support.airtable.com/hc/en-us/articles/4405741487383-Understanding-Airtable-IDs

## Deploy

```
./dockerpush.sh
```

Pushing to Docker Hub will automatically deploy the image to production.
