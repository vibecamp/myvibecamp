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

## Deploy

```
./dockerpush.sh
```

Pushing to Docker Hub will automatically deploy the image to production.
