default: gen-all 

gen-all: api-code-gen-server # api-code-gen-ts api-code-gen-py

api-code-gen-server: #generate/refresh go server code for api.yaml, do this after update the api.yaml
	rm -Rf ./server/genapi ; true
	java -jar openapi-generator-cli-7.14.0.jar generate -i api.yaml -g go -o server/genapi/ -p packageName=genapi -p generateInterfaces=true -p isGoSubmodule=false --git-user-id iworkflowio --git-repo-id ipubsub
	gofmt -s -w server/genapi;
	rm -Rf ./server/genapi/go*; 
	rm -Rf ./server/genapi/git_push.sh;
	rm -Rf ./server/genapi/README.md;
	rm -Rf ./server/genapi/test;
	rm -Rf ./server/genapi/docs;
	rm -Rf ./server/genapi/api;
	rm -Rf ./server/genapi/.*;
	true

#api-code-gen-ts: #generate/refresh typescript apis
#	rm -Rf ./ts-api/src/api-gen ; true
#	java -jar openapi-generator-cli-7.14.0.jar generate -i api.yaml -g typescript-axios -o ./ts-api/src/api-gen --git-user-id xcherryio --git-repo-id apis

#api-code-gen-py: #generate/refresh python apis
#	rm -Rf ./pyapi/* ; true
#	java -jar openapi-generator-cli-7.14.0.jar generate -i api.yaml -g python -o ./pyapi -p packageVersion=0.0.3 -p packageName=xcherryapi --git-user-id xcherryio --git-repo-id apis

