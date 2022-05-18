import grpc from 'k6/net/grpc';
import { check, sleep, group } from 'k6';
import http from "k6/http";
import { FormData } from "https://jslib.k6.io/formdata/0.0.2/index.js";
import { randomString } from "https://jslib.k6.io/k6-utils/1.1.0/index.js";
import { URL } from "https://jslib.k6.io/url/1.0.0/index.js";

import {
    genHeader,
    base64_image,
} from "./helpers.js";

const client = new grpc.Client();
client.load(['proto'], 'model_definition.proto');
client.load(['proto'], 'model.proto');
client.load(['proto'], 'model_service.proto');

const apiHost = "http://localhost:8083";
const cls_model = open(`${__ENV.TEST_FOLDER_ABS_PATH}/integration-test/data/dummy-cls-model.zip`, "b");
const model_def_name = "model-definitions/local"


export function UpdateModel() {
    // UpdateModel check
    group("Model API: UpdateModel", () => {
        client.connect('localhost:8083', {
            plaintext: true
        });

        let fd_cls = new FormData();
        let model_id = randomString(10)
        let model_description = randomString(20)
        fd_cls.append("name", "models/" + model_id);
        fd_cls.append("description", model_description);
        fd_cls.append("model_definition_name", model_def_name);
        fd_cls.append("content", http.file(cls_model, "dummy-cls-model.zip"));
        check(http.request("POST", `${apiHost}/v1alpha/models:multipart`, fd_cls.body(), {
            headers: genHeader(`multipart/form-data; boundary=${fd_cls.boundary}`),
        }), {
            "POST /v1alpha/models:multipart task cls response status": (r) =>
                r.status === 201,
            "POST /v1alpha/models:multipart (multipart) task cls response model.name": (r) =>
                r.json().model.name === `models/${model_id}`,
            "POST /v1alpha/models:multipart (multipart) task cls response model.uid": (r) =>
                r.json().model.uid !== undefined,
            "POST /v1alpha/models:multipart (multipart) task cls response model.id": (r) =>
                r.json().model.id === model_id,
            "POST /v1alpha/models:multipart (multipart) task cls response model.description": (r) =>
                r.json().model.description === model_description,
            "POST /v1alpha/models:multipart (multipart) task cls response model.model_definition": (r) =>
                r.json().model.model_definition === model_def_name,
            "POST /v1alpha/models:multipart (multipart) task cls response model.configuration": (r) =>
                r.json().model.configuration !== undefined,
            "POST /v1alpha/models:multipart (multipart) task cls response model.visibility": (r) =>
                r.json().model.visibility === "VISIBILITY_PRIVATE",
            "POST /v1alpha/models:multipart (multipart) task cls response model.owner": (r) =>
                r.json().model.user === 'users/local-user',
            "POST /v1alpha/models:multipart (multipart) task cls response model.create_time": (r) =>
                r.json().model.create_time !== undefined,
            "POST /v1alpha/models:multipart (multipart) task cls response model.update_time": (r) =>
                r.json().model.update_time !== undefined,
        });

        let res = client.invoke('instill.model.v1alpha.ModelService/UpdateModel', {
            model: {
                name: "models/" + model_id,
                description: "new_description"
            },
            update_mask: "description"
        })
        check(res, {
            "UpdateModel response status": (r) => r.status === grpc.StatusOK,
            "UpdateModel response model.name": (r) => r.message.model.name === `models/${model_id}`,
            "UpdateModel response model.uid": (r) => r.message.model.uid !== undefined,
            "UpdateModel response model.id": (r) => r.message.model.id === model_id,
            "UpdateModel response model.description": (r) => r.message.model.description === "new_description",
            "UpdateModel response model.model_definition": (r) => r.message.model.modelDefinition === model_def_name,
            "UpdateModel response model.configuration": (r) => r.message.model.configuration !== undefined,
            "UpdateModel response model.visibility": (r) => r.message.model.visibility === "VISIBILITY_PRIVATE",
            "UpdateModel response model.owner": (r) => r.message.model.user === 'users/local-user',
            "UpdateModel response model.create_time": (r) => r.message.model.createTime !== undefined,
            "UpdateModel response model.update_time": (r) => r.message.model.updateTime !== undefined,
        });

        check(client.invoke('instill.model.v1alpha.ModelService/DeleteModel', { name: "models/" + model_id }), {
            'Delete model status is OK': (r) => r && r.status === grpc.StatusOK,
        });
        client.close();
    });
};