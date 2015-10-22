# T-Short: A link shortener

T-Short is a link shortener written in Go using PostgreSQL as a database backend
for the lookup function. It is in early development and meant as a learning
experiment. I am open to feedback. Please feel free to submit pull requests or
fork this project.

## Features
### Web UI
At this point, you can submit shortening requests through a simple web form. A
response will be given on another very plain looking page.

###API
There is a basic API for this project as well. Submit POST data:
```
~$ curl -X POST -d "url=https://github.com/samdemorest/tshort" 127.0.0.1:8080
```

The response then will be a JSON formatted string with one parameter, the
shortened URL that will redirect to the full URL that was submitted in the
request.
