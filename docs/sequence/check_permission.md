# Permission check (when create project with flow)

```mermaid
sequenceDiagram
autonumber

actor user
participant browser
participant flowServer as reearth-flow server
participant flowDB as reearth-flow DB
participant dashboardServer as reearth-dashboard server
participant accountDB as account DB
participant cerbos as cerbos server

user ->>+ browser :create project
browser ->>+ flowServer :Request to create project
flowServer ->>+ dashboardServer :Request to check permittion
note over flowServer, dashboardServer: header: access_token, body: {service:"flow", resource: "project", action: "edit"}
dashboardServer ->>+ dashboardServer :Get user_id from access_token
dashboardServer ->>+ accountDB :Get role_ids from permittable table based on user_id
accountDB ->>+ dashboardServer :return
dashboardServer ->>+ accountDB :Get role_names from role table based on role_ids
accountDB ->>+ dashboardServer :return
dashboardServer ->>+ cerbos :Request to check permittion
note over dashboardServer, cerbos: {service:"flow", resource: "project", action: "edit", roles: ["role1"]}
cerbos ->>+ cerbos :Check permissions using flow_project policy file
cerbos ->>+ dashboardServer :OK
dashboardServer ->>+ flowServer :OK
flowServer ->>+ flowDB: create project
flowDB ->>+ flowServer :OK
flowServer ->>+ browser :OK
browser -->>+ user: OK
```
