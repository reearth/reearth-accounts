# リソース単位のロール割り当て（汎用的なリソース単位メンバーシップ）

> **Status: DRAFT.** 本ドキュメントは [resource-role-binding.md](resource-role-binding.md)
> の日本語版です。内容に差異がある場合は英語版を正とします。
>
> project 単位で（より一般には、各サービスのリソース単位で）メンバーとロールを設定できるようにします。
> これにより各サービス（dashboard、visualizer、cms など）は、メンバー管理の仕組みをそれぞれ作らずに
> project member を持てます。その際、**reearth-accounts は project が何であるかを知りません**。
> 最初に使うのは Dashboard の project です。コード上では、ロールの割り当ては `...Binding` /
> `resource_bindings` という名前の型・テーブルで表します。

## Document Signature

|           |                |
|-----------|----------------|
| Creator   | soneda-yuya    |
| Leader    | TBD            |
| Task Link | TBD            |
| Developer | TBD            |

## Background / Problem Statement

### 動機（プロダクト上の背景）

- SSO の導入にあたり「ワークスペースを自由に作成できるべきか」を議論し、
  オプションでワークスペースの作成を制限できるべきという意見が出ました。
  制限すると、1 つのワークスペースで多数の project を管理することになります。
- 現状、メンバーは workspace 単位でしか割り当てられないため、顧客は workspace ごとに
  メンバーを管理しています。ただし、project ごとに関係するメンバーは異なります。
  workspace を跨いで project を共有したい場合、顧客は現在 project を対象の
  workspace へコピーして対応しています。
- project ごとにメンバーと権限を割り当てられるようにすることで、workspace を
  組織として機能させられます。SSO / SCIM とは別の機能ですが、ワークスペースの作成を
  制限できるようになった時点で使えるよう、早めに実装できる状態にしたいと考えています。
- 目指すモデルは GitHub に近いものです。workspace と project の関係は、GitHub の
  organization と repository の関係に相当します。GitHub に organization member と
  repository member の両方があるのと同様に、Re:Earth にも workspace member と
  project member の両方を持たせます。

### 現状

- Re:Earth の権限は reearth-accounts が判定しています。各サービス（dashboard、visualizer、
  cms など）は、操作を許可する前に「このユーザーはこの操作をしてよいか」を accounts に問い合わせます。
- ユーザーのロールは **workspace 単位** で持っています（ほかにプラットフォーム全体のロールが少しあります）。
  workspace に参加すると、その workspace のすべての project に対して同じロールを持つことになります。
- そのため権限チェックで分かるのは「どの workspace か」だけで、「どの project か」は分かりません。
  同じ workspace の中で「user A は project P を編集できるが project Q は編集できない」といった
  指定はできません。

### 課題

プロダクトでは **project 単位のメンバーシップ** が必要です。たとえば
「user A は project P の editor、同じ workspace の user B は P を見られない」といったものです。
現在 accounts が表現できる最も細かい単位は workspace です。

各サービスが project のメンバーシップを独自に実装すると、次の問題が起きます。

1. 招待・ロール変更・削除の API と UI がサービスごとに重複する。
2. 各サービスは、project member を掃除するために **workspace メンバーシップの変更**
   （workspace からのユーザー削除、workspace の削除）に反応しなければならない。
   その情報を持つのは accounts だけなので、サービスごとにイベントや同期の仕組みが必要になる。
   取りこぼすと、すでに抜けた workspace の project にユーザーがアクセスできてしまう。
3. 「project member は workspace のメンバーでなければならない」という制約を、
   結局 accounts を呼ばずにサービス内だけで検証することはできない。
4. 権限のルールが 1 か所（accounts）で判定されず、各サービスにばらばらに散らばる。

### 制約（この設計の形を決めている理由）

accounts は *project* が何であるかを取り込んではいけません。project の存在、名前、
公開範囲、中身、ライフサイクルは引き続き各サービスが持ちます。accounts が持つのは、
権限の判定に必要な「この project はこの workspace に属し、このユーザーはそれに対して
このロールを持つ」という情報だけです。

## Goals

1. **project ごとにメンバーとロールを設定できる。** workspace 全体だけでなく、特定の project
   に対してロールを与えられる（例: project P は編集できるが、project Q にはアクセスできない）。
2. **権限チェックで、対象の project を考慮できる。** サービスが accounts に「このユーザーは
   この操作をしてよいか」と問い合わせたとき、その project でのユーザーのロールを踏まえて判定できる。
   判定には常にその project が実際に属する workspace を使うため、呼び出し側が別の workspace を
   指定してアクセスを得ることはできない。
3. **メンバーの整合性が保たれる。** project は必ず1つの workspace に属し、一度 owner が
   設定されたら owner が0人になることはない。複数人が同時にメンバーを変更しても崩れない。
4. **workspace から外れると project へのアクセスもなくなる。** ユーザーを workspace から外すと、
   その workspace 内のすべての project でのロールも自動で取り消される。各サービスが個別に
   後始末をする必要はない。
5. **accounts は汎用的なままにする。** accounts は「project」が何かを知らない。どのサービスも、
   権限ポリシーを追加するだけで、自分のリソース（例: CMS のモデル）に同じ仕組みを使える。
   accounts の変更は不要。
6. **既存の動作を壊さない。** 現在の権限チェックとポリシーは、これまでとまったく同じように動く。

## Non-Goals

1. **project の詳細情報を accounts に保存すること。** 名前、公開範囲、所有者、日時など
   project の情報は、引き続きそれを持つサービスが管理する。
2. **project が実在するかを確認すること。** accounts は project を登録したサービスを信頼する。
   project を登録できるのはサービスだけで、ユーザーは登録できない。
3. **project の削除を accounts が自分で検知すること。** project が削除されたら、
   それを持つサービスが accounts に知らせる。
4. **入れ子のリソースや権限の継承**（例: folder → project → layer）。後から権限ポリシーで
   追加できるが、今回は対象外。
5. **integration を project のメンバーにすること。** 最初のバージョンでは、project のメンバーに
   なれるのは人（ユーザーアカウント）だけで、integration や API トークンはなれない。
6. **project の属性（例: 公開 / 非公開）を権限の判定に使うこと。** *Open Questions* で扱う。
   最初のバージョンはロールだけで判定する。
7. **project メンバーシップの招待メールを送ること。** メールには project 名が入るが、
   accounts はそれを知らないため、サービス側で送る。

## Functional Requirements

1. **権限チェックの速さを保つ。** project を指定しない権限チェックの負荷は今とまったく同じ。
   project を指定するチェックで増えるのは、その project がどの workspace に属するかを調べる
   小さく速い参照1回だけ。ユーザーの project ロールは、accounts がすでに読み込んでいるデータと
   一緒に読む。
2. **名前と ID に上限を設ける。** リソースの種別名とリソース ID には最大長と使える文字の制限があり、
   保存するデータを小さく予測しやすい状態に保つ。
3. **現実的な規模を想定する。** 1ユーザーあたりの project ロールの割り当ては約 1,000 件まで、
   1 project あたりのメンバーは約 1,000 人まで。
4. **project のメンバー一覧をすばやく取得できる。** 返すのはメンバーが誰かとそのロールだけで、
   名前などの詳細はサービス側が持つ。
5. **accounts が対応するすべてのデータベースで動く。** どのデータベースを使う環境でも、
   メンバーの変更は同じように動作する。

技術的な詳細: *Solution Options → Option 1 → 詳細な要件* を参照。

## Solution Options

この節ではシステム設計を詳しく説明します。実装に関心のある方向けの内容で、ほかの節は
この節を読まなくても理解できます。

### Option 1（推奨）: リソース登録 + `Permittable` に埋め込んだロールの割り当て

2つの状態を持ちます。どちらも accounts にとっては中身を解釈しないものです。

| 状態 | 目的 |
|---|---|
| **リソース登録** `(resourceKind, resourceID) → workspaceID, version` | リソースの workspace の範囲を正として持つ。所有するサービスが登録したことの証明。不変条件のためのリソース単位の直列化ポイント |
| `Permittable` 上の **リソースロール** `(workspaceID, resourceKind, resourceID, roleID)` | そのリソースに対するユーザーのロール。permittable と一緒に読むので、参照のための追加読み込みは不要 |

#### ドメイン

```go
// server/pkg/resource/registration.go (new package, no service-specific code)
type Registration struct {
    createdAt   time.Time
    kind        Kind   // validated per FR-3
    resourceID  ID     // validated per FR-3
    version     int64  // bumped by every membership mutation of this resource
    workspaceID workspace.ID
}

// server/pkg/permittable/permittable.go
type Permittable struct {
    id             ID
    resourceRoles  []ResourceRole // NEW
    roleIDs        []role.ID
    updatedAt      time.Time
    userID         user.ID
    workspaceRoles []WorkspaceRole
}

type ResourceRole struct {
    resourceID   resource.ID
    resourceKind resource.Kind
    roleID       role.ID
    workspaceID  workspace.ID // copied from the registration
}
```

`ResourceRole(kind, id)` は、**ユーザーがまだ `workspaceID` の workspace ロールを持っている場合に限り**、
そのリソースに対するユーザーのロールを返します（読み込み時のガード。*並行性* を参照）。
ロールはユーザーごとに `(kind, id)` 1つにつき1つです。accounts のどこにも、`kind` で
`switch` する処理はありません。

ロールは既存の `roles` テーブル（`owner` / `maintainer` / `writer` / `reader`）を再利用します。
リソース種別ごとのロールの意味は、その種別の Cerbos ポリシーだけで定義します。

#### ストレージ

**PostgreSQL**: マイグレーション `0005_resource_role_binding.up.sql`

```sql
CREATE TABLE resource_registrations (
    resource_kind text NOT NULL CHECK (octet_length(resource_kind) <= 64),
    resource_id   text NOT NULL CHECK (octet_length(resource_id) <= 128),
    workspace_id  text NOT NULL,
    version       bigint NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (resource_kind, resource_id)
);
CREATE INDEX resource_registrations_workspace_idx
    ON resource_registrations (workspace_id);

CREATE TABLE permittable_resource_roles (
    permittable_id text NOT NULL REFERENCES permittables(id) ON DELETE CASCADE,
    resource_kind  text NOT NULL,
    resource_id    text NOT NULL,
    workspace_id   text NOT NULL,
    role_id        text NOT NULL,
    PRIMARY KEY (permittable_id, resource_kind, resource_id),
    FOREIGN KEY (resource_kind, resource_id)
        REFERENCES resource_registrations (resource_kind, resource_id) ON DELETE CASCADE
);
CREATE INDEX permittable_resource_roles_resource_idx
    ON permittable_resource_roles (resource_kind, resource_id);
CREATE INDEX permittable_resource_roles_workspace_idx
    ON permittable_resource_roles (workspace_id, permittable_id);
```

```sql
-- 0005_resource_role_binding.down.sql
DROP TABLE IF EXISTS permittable_resource_roles;
DROP TABLE IF EXISTS resource_registrations;
```

**MongoDB**

- 新しいコレクション `resource_registrations` を追加します。`{ resource_kind: 1, resource_id: 1 }`
  に一意インデックス、`{ workspace_id: 1 }` にインデックスを張ります
  （スキーマは `mongoschemagen` で登録し、`server/CLAUDE.md` の手順に従ってマイグレーションを追加）。
- permittable ドキュメントに任意の配列 `resource_roles` を追加します。インデックスは
  `{ "resource_roles.resource_kind": 1, "resource_roles.resource_id": 1 }` と
  `{ "resource_roles.workspace_id": 1 }` です。

#### リポジトリの変更（スナップショットではなく対象を絞った書き込み）

既存のスナップショット保存 `Save` / `SaveMany` が、リソースロールを上書きしたり
復活させたりできないようにします。

- **Mongo** の `Permittable.Save` は、ドキュメントの置き換えから、`role_ids`、`workspace_roles`、
  `updated_at` だけの `$set` に切り替えます。`resource_roles` は下記の対象を絞った操作だけが書き込みます。
- **Postgres** の `Permittable.Save` はもともと `permittable_workspace_roles` しか書き直さないので、
  リソースロールについては手を加えません。
- **Postgres** の `hydrate` は、ベースの行に加えて workspace ロールとリソースロールを
  `json_agg` のサブセレクトで返す1本のクエリに置き換えます（FR-2）。

新しいリポジトリのインターフェース:

```go
// resource.RegistrationRepo
FindByKey(ctx, kind, id) (*resource.Registration, error)
Create(ctx, r resource.Registration) error              // ErrAlreadyExists on duplicate
LockForUpdate(ctx, kind, id) (*resource.Registration, error) // must run inside a transaction
Remove(ctx, kind, id) error                              // also removes all role assignments

// permittable.Repo (additions)
FindResourceMembers(ctx, kind, id) ([]permittable.ResourceMember, error) // projection: userID + roleID
CountResourceOwners(ctx, kind, id, ownerRoleID) (int, error)
UpsertResourceRole(ctx, userID, r permittable.ResourceRole) error
DeleteResourceRole(ctx, userID, kind, id) error
DeleteResourceRolesInWorkspace(ctx, workspaceID, userIDs) error
```

| 操作 | Mongo | Postgres |
|---|---|---|
| `UpsertResourceRole` | 単一ドキュメントの更新: `(kind,id)` の要素を `$pull` してから新しい要素を `$push`（集約パイプラインを使った1回の `updateOne`） | `INSERT … ON CONFLICT (permittable_id, resource_kind, resource_id) DO UPDATE SET role_id = …` |
| `DeleteResourceRole` | `$pull` | `DELETE … WHERE permittable_id=$1 AND resource_kind=$2 AND resource_id=$3` |
| `DeleteResourceRolesInWorkspace` | `updateMany` + `$pull {workspace_id}` | `DELETE … WHERE workspace_id=$1 AND permittable_id = ANY($2)` |
| `FindResourceMembers` | 集約 `$match` → `$unwind` → `$match` → `$project {user_id, role_id}` | `SELECT p.user_id, r.role_id FROM permittable_resource_roles r JOIN permittables p …` |
| `LockForUpdate` | トランザクション内で `findOneAndUpdate {$inc: {version: 1}}`（同じ登録に対する並行トランザクションは書き込み競合となり、リトライされる） | `SELECT … FOR UPDATE` |

#### 権限チェック

`CheckPermissionInput` に任意項目を1つ追加します。

```graphql
input CheckPermissionInput {
  service: String!
  resource: String!
  action: String!
  workspaceAlias: String
  resourceId: String   # NEW
}
```

`resourceId` が **ない** 場合、挙動は変わりません。

`resourceId` が **ある** 場合:

1. 登録 `(service:resource, resourceId)` を読み込む。**見つからなければ拒否。**
2. `workspaceAlias` が指定されていて、それが登録の `workspaceID` に解決されなければ **拒否**。
3. principal のロール = グローバルロール + **登録の `workspaceID`** におけるユーザーの workspace ロール
   （呼び出し元は workspace を選べない）。
4. principal の属性 `resourceRole` = このリソースでユーザーに割り当てられたロール。
   割り当てがない場合（または読み込み時のガードで除外された場合）は `""`。
5. Cerbos のリソース ID は実際の `resourceId` にする。

ポリシーは引き続き各サービスの `definition.go` に置き、reearthx の generator がすでに
サポートしている条件を使います。`dashboard:project` の例:

```go
// dashboard: definition.go
resourceRole := func(expr string) *generator.Condition { return generator.SimpleExpr(expr) }
any := []string{"*"} // any principal role; the condition does the gating

builder.AddResource("project", []generator.ActionDefinition{
    // workspace admins keep full access to every project in their workspace
    generator.NewActionDefinition("read", []string{"owner", "maintainer"}),
    generator.NewActionDefinition("edit", []string{"owner", "maintainer"}),
    generator.NewActionDefinition("delete", []string{"owner", "maintainer"}),
    generator.NewActionDefinition("manage_members", []string{"owner", "maintainer"}),
    // project members
    generator.NewActionDefinitionWithCondition("read", any, resourceRole(`P.attr.resourceRole != ""`)),
    generator.NewActionDefinitionWithCondition("edit", any, resourceRole(`P.attr.resourceRole in ["owner", "writer"]`)),
    generator.NewActionDefinitionWithCondition("delete", any, resourceRole(`P.attr.resourceRole == "owner"`)),
    generator.NewActionDefinitionWithCondition("manage_members", any, resourceRole(`P.attr.resourceRole == "owner"`)),
})
```

リソースロールは principal のロールではなく principal の **属性** として渡されるので、
project の `owner` が workspace の `owner` と取り違えられることはありません。workspace ロールが
project メンバーシップに *加算される* のか、project メンバーシップによって *制限される* のかは、
プロダクトごとのポリシーで決めます。

#### メンバー管理 API

**ライフサイクル（M2M のみ）**: REST のサービス用ルートで、M2M の API キーで **のみ** 認証します
（ユーザーの JWT は拒否）。登録は、リソースが存在することとそのリソースがどの workspace に属するかを
accounts が知る手段です。登録をサービスに限定することで、ユーザーが未登録の ID を
自分のものとして主張する手段をなくします。

```
POST   /api/service/resources
       { workspaceId, resourceKind, resourceId, ownerUserId? }
       → creates the registration and, if ownerUserId is given, an `owner`
         role assignment, in one transaction. ownerUserId must be a workspace member.
         409 if already registered with a different workspace; idempotent
         (200) if registered with the same workspace.
DELETE /api/service/resources/{resourceKind}/{resourceId}
       → removes the registration and every role assignment on the resource.
```

**メンバーシップ（ユーザーの JWT、GraphQL）**:

```graphql
input SetResourceMemberInput {
  resourceKind: String!
  resourceId: String!
  userId: ID!
  role: Role!
}
input RemoveResourceMemberInput {
  resourceKind: String!
  resourceId: String!
  userId: ID!
}

type ResourceMember { userId: ID!, role: Role! }
type ResourceBinding { workspaceId: ID!, resourceKind: String!, resourceId: String!, role: Role! }

extend type Query {
  resourceMembers(resourceKind: String!, resourceId: String!): [ResourceMember!]!
  # IDs only; the owning service hydrates names etc.
  myResourceBindings(resourceKind: String!, workspaceId: ID): [ResourceBinding!]!
}

extend type Mutation {
  setResourceMember(input: SetResourceMemberInput!): ResourceMember
  removeResourceMember(input: RemoveResourceMemberInput!): Boolean!
}
```

ガード（汎用的で、種別固有のコードはなし）。すべての更新操作は、登録に対する `LockForUpdate`
から始まる1つのトランザクションで実行します。

| 操作 | ガード（ロック取得後に評価） |
|---|---|
| `setResourceMember` | 登録が存在する。対象ユーザーが登録の workspace のメンバーである。操作者に `(kind, id)` への `manage_members` が Cerbos で許可されている。変更が最後の `owner` を降格させる場合は拒否 |
| `removeResourceMember` | Cerbos の `manage_members`、または操作者 == 対象ユーザー（自分で抜ける場合）。最後の `owner` を外す場合は拒否 |
| `resourceMembers` | `(kind, id)` への Cerbos の `read`。結果は読み込み時のガードで絞り込む |
| `myResourceBindings` | 操作者自身のロールの割り当てのみ。読み込み時のガードで絞り込む |

**最後の owner のルール:** リソースの owner 数を 1 以上から 0 にしてしまうメンバーシップの更新は
拒否します。owner なしで登録されたリソース（例: 埋め戻し）は、owner が設定されるまで workspace ロールで
管理します。登録解除のエンドポイントはこのルールの対象外です。

#### 並行性

| 競合 | 対策 |
|---|---|
| 同じリソースの別々のユーザーに対する2つの書き込みが、どちらも最後の owner のチェックを通過する | どちらも同じ登録に対して `LockForUpdate` を取る必要があり、後の方は待つ（Postgres）か、競合してリトライする（Mongo）。owner 数はロック取得後に読む。 |
| 2つのサービスが同じ `(kind, id)` を登録する | 主キー / 一意インデックスにより、負けた方は `ErrAlreadyExists` になる。 |
| `setResourceMember` が workspace メンバーシップのチェックを通過した後、ロールの割り当てが書き込まれる前にユーザーが workspace から外され、連動削除が実行される | **読み込み時のガード**: ロールの割り当ては、ユーザーが現在その割り当ての workspace で workspace ロールを持っている場合にのみ有効とする。permittable はすでに読み込まれているので、追加の読み込みはかからない。残った孤立行は無害で、次に連動削除や登録解除のエンドポイントがその行に触れたときに削除される。 |
| workspace フローからの古いスナップショットの `Save` が、新しいロールの割り当てを上書きする（またはその逆） | スナップショット保存は `resource_roles` を書き込まず、ロールの割り当ての書き込みは `workspace_roles` を書き込まない（*リポジトリの変更* を参照）。 |

#### ライフサイクルと連動削除

| イベント | 実行者 | ロールの割り当てへの影響 |
|---|---|---|
| リソース作成 | サービス（M2M） | `ownerUserId` = 作成者として `POST /api/service/resources` を呼ぶ |
| リソース削除 | サービス（M2M） | `DELETE /api/service/resources/{kind}/{id}` を呼ぶ。呼び忘れると孤立した登録が残るが無害（そのリソースはサービス側にもう存在しない）。後から任意の掃除の仕組みで削除する |
| 既存のリソース（本機能以前） | サービス（M2M） | サービスが `resourceId` を送り始める前に、同じエンドポイント（冪等）で一度だけ埋め戻す |
| workspace からユーザーを削除 | accounts | `bulkRemovePermittable` から `DeleteResourceRolesInWorkspace` も呼ぶ |
| workspace 削除（`Workspace.Remove`） | accounts | 同じく `bulkRemovePermittable` 経由。その workspace の登録も削除する |
| workspace の無効化 / 復元 | accounts | 変更なし（workspace ロールと同じ。復元に備えてデータを残す） |
| ユーザー削除（`DeleteMe`） | accounts | 現状、`DeleteMe` は permittable を削除しない。ユーザーが消えればロールの割り当てには到達できなくなる。`DeleteMe` で permittable を削除する修正は、本機能とは独立したフォローアップとして提案する |

**メリット**

- リソースの workspace の範囲はサーバー側で正として管理されるので、呼び出し元が
  別の workspace のロールを借りることはできない。
- accounts にリソースを存在させられるのは M2M 認証されたサービスだけなので、
  ユーザーが ID を自分のものとして主張することはできない。
- リソース全体の不変条件は、両バックエンドでリソースごとに1つのレコードで直列化される。
- ロールの割り当ての参照はすでに読み込まれた permittable の中で完結する。新たな読み込みは
  登録の主キー参照だけ。
- accounts はドメインに依存しないまま: 種別と ID は解釈しない値で、意味はポリシーが持つ。

**デメリット**

- `resourceId` を渡す `checkPermission` 呼び出しでポイント読み込みが1回増える。
- サービスはリソースを登録し（埋め戻しを含む）、削除時に登録を解除する必要がある。
- Mongo の `Save` の意味が置き換えから `$set` に変わる（既存コードに手が入る）。

#### 詳細な要件

*Functional Requirements* の技術的な形です。各項目は順に FR-1〜FR-6 で、この節のほかの部分では
この番号で参照します。

1. `resourceId` なしの `checkPermission`: 読み込みは **変わらない**。
   `resourceId` ありの場合: **インデックスが効くポイント読み込みが1回増える**
   （主キーによるリソース登録の読み込み）。ロールの割り当て自体は、すでに読み込まれた permittable から読む。
2. PostgreSQL では、`Permittable.FindByUserID` がベースの行、workspace ロール、リソースロールを
   **1回のラウンドトリップ** で読み込む（集約したサブセレクト）。そのため、リソースロールを追加しても
   `checkPermission` のクエリは増えない。
3. 項目の上限。ドメイン層と DB 制約の両方で強制する:
   - `resourceKind`: 64 バイト以下、`^[a-z0-9-]{1,32}:[a-z0-9_-]{1,31}$`
   - `resourceID`: 128 バイト以下、`^[A-Za-z0-9_.:-]{1,128}$`
4. 想定規模: 1ユーザーあたりのロールの割り当ては約 1,000 件以下（上記の上限で1件約 300 B 以下、
   つまり Mongo の1ドキュメントあたり約 300 KB 以下で、16 MB を大きく下回る）。
   1リソースあたりのメンバーは約 1,000 人以下。
5. 1リソースのメンバー一覧取得はインデックスが効き、そのリソースの `(userId, role)` だけを返す。
   集約全体を組み立てることはしない。
6. すべての更新操作が MongoDB / PostgreSQL の両バックエンドと `memory` リポジトリで動くこと。

範囲についての補足（*Non-Goals* の技術的な形）:

- リソースの存在は検証しない。accounts はリソースを登録する **M2M 認証された** サービスを信頼し、
  リソースの削除時に登録解除のエンドポイントを呼ぶこともそのサービスに任せる
  （*ライフサイクルと連動削除* を参照）。
- v1 のロールの割り当ては **ユーザーの principal のみ**。integration（M2M）の principal は
  リソースロールを持てない。
- v1 の `checkPermission` は、リソースの属性（例: `visibility`）を Cerbos に渡さない。
  ポリシーはロールだけを使う（*Open Questions の技術的な補足* を参照）。

#### リクエストの流れ（シーケンス図）

##### resource ID を指定した権限チェック

```mermaid
sequenceDiagram
    participant S as Service (e.g. dashboard)
    participant A as reearth-accounts
    participant DB as Mongo/Postgres
    participant C as Cerbos

    S->>A: checkPermission(service, resource, action, workspaceAlias?, resourceId)
    A->>DB: Permittable.FindByUserID(user)  (existing read, one round-trip)
    A->>DB: Registration.FindByKey(kind, resourceId)  (new point read)
    alt not registered, or alias ≠ registration.workspace
        A-->>S: { allowed: false }
    else
        A->>A: roles = global + workspace roles in registration.workspace<br/>attr.resourceRole = guarded assigned role or ""
        A->>C: CheckResources(principal{roles, attr}, resource{kind, resourceId}, action)
        C-->>A: ALLOW / DENY
        A-->>S: { allowed }
    end
```

##### project の作成とメンバー招待

```mermaid
sequenceDiagram
    participant U as User
    participant S as Service
    participant A as reearth-accounts

    U->>S: createProject
    S->>S: persist project P
    S->>A: POST /api/service/resources {ws, "dashboard:project", P, owner=U} (M2M key)
    A->>A: U ∈ ws? → create registration + owner role assignment (one tx)
    A-->>S: 201
    U->>S: invite B as writer
    S->>A: setResourceMember(kind, P, B, writer) (U's JWT)
    A->>A: lock registration → B ∈ ws? Cerbos manage_members for U? → upsert
    A-->>S: ok
```

##### workspace メンバー削除時の連動

```mermaid
flowchart LR
    R[removeUserFromWorkspace / deleteWorkspace] --> B[bulkRemovePermittable]
    B --> W[RemoveWorkspaceRole ws → snapshot Save]
    B --> RR[DeleteResourceRolesInWorkspace ws, users → targeted delete]
```

#### 技術的な補足: 影響・テスト・デプロイ・ロールバック・監視

##### 影響

1. **permittable のドキュメント / 行のサイズ** がロールの割り当て1件あたり約 300 B 以下増える。
2. **Mongo の `Permittable.Save`** が置き換えから `$set` に変わる。permittable の永続化に関する
   既存テストは引き続き通る必要があり、`Save` が `resource_roles` に触れないことを確認する
   新しいテストを追加する。
3. `resourceId` を渡した場合の **Cerbos のリソース ID の変更**: 影響を受けるのは、利用を始めた
   呼び出し元だけ。
4. **workspace メンバー削除** で、対象を絞った削除が1回増える。
5. **ポリシーの書き方**: ロール名は workspace とリソースで共通。ポリシーは、リソースへのアクセスを
   principal のロールではなく `P.attr.resourceRole` で制御しなければならない。
6. **gqlclient**（`server/pkg/gqlclient`）の利用側は、新しいフィールドを使うためにバージョンを
   上げる必要がある。古いクライアントには影響しない。

##### テスト

1. **ユニット**（`pkg/resource`、`pkg/permittable`）: 種別 / ID の検証と長さの上限。
   リソースごとにロールは1つ。読み込み時のガードが、対応する workspace ロールのない割り当てを除外すること。
2. **interactor**（`internal/usecase/interactor`）:
   - `CheckPermission`: `resourceId` なし → roles が変わらない（リグレッション）。
     未登録のリソース → 拒否。別の workspace の alias → 拒否。workspace ロールは alias ではなく
     登録から取られる。`resourceRole` が設定される / 空になる。
   - 登録: M2M のみ（JWT は拒否）。owner は workspace のメンバーでなければならない。
     同じ workspace での重複は冪等、別の workspace なら 409。
   - `setResourceMember` / `removeResourceMember`: メンバーでない対象ユーザーは拒否。
     `manage_members` がなければ拒否。自分で抜けるのは許可。最後の owner の保護。
   - 連動削除: `RemoveUserMember` / `RemoveMultipleUserMembers` / `Remove`。
3. **並行性**（`make test-integration`、両バックエンド）: 2つの goroutine が最後の2人の owner を
   同時に外す → ちょうど1つだけ成功する。同じキーの並行登録 → 1つだけ成功する。
   `UpsertResourceRole` の後の古いスナップショットの `Save` でもロールの割り当てが残る。
4. **リポジトリ**: すべての新しい操作の Mongo・Postgres・memory 実装。Postgres の
   `FindByUserID` が1本のクエリしか発行しないこと。
5. **マイグレーション**: 既存 DB に対する `0005` の up / down。既存の permittable が
   空の `resourceRoles` で読み込めること。
6. **E2E**（`server/e2e`）: 上記の条件で生成した `dashboard:project` のテスト用ポリシーを使い、
   メンバー / 非メンバー / workspace owner / 別 workspace の結果を確認する。

##### デプロイ

1. 後方互換性はあるか？ **あり。** 入力の任意項目、新しいコレクション / テーブル、
   新しいルートと GraphQL フィールドの追加のみ。`resourceId` がなければ挙動は変わらない。
2. 部分的にデプロイできるか？ **できる。** 順序:
   1. accounts をデプロイする。
   2. 各サービスが既存リソースの登録を埋め戻す。
   3. 各サービスが作成時の登録 / 削除時の登録解除を始める。
   4. 各サービスが `P.attr.resourceRole` を使うポリシーをデプロイし、`resourceId` を送り始める。
      ステップ 4 より前は属性が存在せず、条件がマッチしない（安全側に倒れる）。
3. 通知先: dashboard / visualizer / cms のオーナー、Cerbos ポリシーの管理者。
4. 設定変更: なし（既存の M2M API キーを再利用する）。
5. DDL: Postgres のマイグレーション `0005` は起動時に自動で実行される（golang-migrate）。
   Mongo は起動時のマイグレーションでコレクションのスキーマとインデックスを作成する。
   accounts 側での埋め戻しはない。

##### ロールバック

1. 原因をチームの Slack チャンネルに共有する。
2. `resourceId` / `resourceRole` に依存する利用側サービス / ポリシーを先に戻す。
3. accounts のイメージを戻す。旧バイナリは新しいコレクション、テーブル、`resource_roles`
   フィールドを無視するので、**データのロールバックは不要**。
   注意: 旧バージョンの Mongo の `Save` はドキュメント全体を置き換えるため、書き込んだ permittable の
   `resource_roles` は消える。機能は無効になっているのでロールバック中は許容でき、
   再度ロールフォワードする際にサービスが埋め戻しを再実行する。
4. スキーマ自体を削除する必要がある場合のみ、`0005` の down を実行する
   （登録とロールの割り当てが消えるので、先にダンプを取る）。

##### デプロイ後の確認と監視

- チェックリスト
  - dashboard の staging で project を作成し、登録と作成者への `owner` ロールの割り当てができることを確認する。
  - workspace メンバーを招待してアクセスできることを確認する。そのメンバーを workspace から外し、
    project にアクセスできなくなることを確認する。
  - 別の workspace の alias を指定して `checkPermission` を呼び、拒否されることを確認する。
- メトリクス
  - `resourceId` あり / なしでの `checkPermission` の p50 / p95 レイテンシ。
  - 登録 / 登録解除 / `setResourceMember` のエラー率（Mongo のトランザクションのリトライを含む）。
  - permittable あたりのロールの割り当て数（最大 / p99）。FR-4 の監視用。
- アラート
  - `checkPermission` のエラー率: Warning > 1%、Danger ≥ 5%。
  - `checkPermission` の p95 レイテンシ: Warning > ベースライン比 +20%。

#### Open Questions の技術的な補足

1. **`checkPermission` へのリソース属性の受け渡し:** 提案は、`CheckPermissionInput` に任意の
   `attributes: JSON` を追加し、Cerbos のリソースにそのまま渡すこと（例: `visibility`）。
2. **一括チェック:** 一覧画面向けに、複数の `resourceId` に対する `checkPermissions` を用意する。
   Cerbos の `CheckResources` はすでに複数リソースに対応している。
3. **サービスごとの M2M キー:** v1 では登録 / 登録解除（`POST` / `DELETE /api/service/resources`）に
   既存の共有 M2M API キーを受け付ける。案: 各キーをそのサービス自身の種別（`dashboard:*`）に限定する。
4. **孤立データの掃除（sweeper）:** 登録解除のエンドポイントを呼ばずに削除されたリソースの登録を
   削除する。
5. **integration メンバー:** M2M トークンをリソースの principal にすること（cms で必要になる可能性）。

### Option 2: ロールの割り当てを独立したコレクション / テーブルに置く（埋め込まない）

登録と API は同じですが、ロールの割り当てを permittable ではなく、独立した
`resource_bindings (kind, id, userID, role)` コレクション / テーブルに置きます。

**メリット:** permittable が大きくならない。メンバー一覧は単純なインデックススキャンになる。
スナップショットの `Save` の意味を変える必要がない。

**デメリット:** `checkPermission` で、登録に加えてもう1回（ロールの割り当ての）読み込みが増える。
workspace の連動削除で2つのストアを更新する必要がある。

**結論:** Option 1 を採用します。1ユーザーあたりのロールの割り当て数が FR-4 に近づいた場合、
または Mongo の `Save` の変更にリスクがあると判明した場合は、Option 2 を再検討します。

### 検討して却下した案: 各サービスにメンバーシップを持たせる

*課題* の (1)〜(4) の重複と連動削除の問題があるため却下しました。
「メンバー情報はサービスに置き、ロールを属性として Cerbos に渡す」という派生案は、(4) しか解決しません。

## Design

**accounts が project ごとに持つのは、「どの workspace に属するか」と「誰がどのロールを持つか」の
2つだけ。** それ以外の project の情報はすべてサービス側に残る。

### project を指定した権限チェック

1. サービスが accounts に、ユーザーが特定の project に対してある操作（例: 編集）をしてよいかを
   問い合わせる。
2. accounts は、その project がどの workspace に属するかを調べる。サービスが登録していない
   project の場合や、問い合わせで別の workspace が指定されていた場合は「不可」と答える。
3. accounts は、その workspace でのユーザーのロールと、その project でのユーザーのロール（あれば）
   を合わせ、サービスの権限ポリシーで判定する。
4. project のロールが有効なのは、ユーザーがその project の workspace のメンバーである間だけ。

### project の作成とメンバー招待

1. ユーザーがサービスで project を作成する。サービスは project を保存し、作成者を owner として
   accounts に登録する（作成者は workspace のメンバーでなければならない）。
2. owner が別の workspace メンバーをロール（例: writer）付きで招待する。accounts は、招待される人が
   workspace のメンバーであること、招待する人がメンバーを管理する権限を持つことを確認してから
   ロールを保存する。
3. 同じ project のメンバー変更は1件ずつ順に処理するため、2人が同時に操作しても最後の owner が
   外れることはない。メンバーは自分で project から抜けられるが、最後の owner は抜けられない。
4. owner なしで登録された project（例: 本機能より前からある project）は、owner が設定されるまで
   workspace のロールで管理する。

### workspace からのメンバー削除

ユーザーを workspace から外したり workspace を削除したりすると、accounts はその workspace での
ロールと、その workspace 内のすべての project でのロールを同じ処理の中で取り消す。workspace を
削除した場合は、その workspace の project の登録も削除する。workspace を無効化しても何も消さず、
復元できるようにしておく。

### 例: Dashboard の project

| 誰が | できること |
|---|---|
| workspace の owner / maintainer | workspace 内のすべての project を閲覧・編集・削除し、メンバーを管理できる |
| project の owner | その project を閲覧・編集・削除し、メンバーを管理できる |
| project の writer | その project を閲覧・編集できる |
| その他の project ロール（maintainer / reader） | その project を閲覧できる |
| project のロールを持たない workspace メンバー | その project にアクセスできない（workspace の owner / maintainer を除く） |

workspace ロールが project のメンバーシップに *加算される* のか、project のメンバーシップによって
*制限される* のかは、プロダクトごとに権限ポリシーで決める。

技術的な詳細: *Solution Options → Option 1 → リクエストの流れ（シーケンス図）* を参照。

## Potential Impact

1. **ユーザーあたりの保存データが少し増える。** project のロール1件につき、ユーザーの権限データが
   数百バイト増える。
2. **accounts の権限データの保存方法が一部変わる。** accounts が対応するデータベースの1つで、
   workspace の変更と project ロールの変更が互いに上書きしないよう、権限データの保存方法を変える。
   既存のテストと新しいテストで確認する。
3. **影響を受けるのは利用を始めたサービスだけ。** 権限チェックの挙動が変わるのは、特定の project を
   指定し始めたサービスだけで、それ以外はこれまでどおり。
4. **workspace メンバーの削除で処理が少し増える。** その workspace での project ロールも同じ処理で
   取り消すため。
5. **ロール名は workspace と project で共通。** 「owner」「maintainer」「writer」「reader」は
   どちらにもある。サービスの権限ポリシーを書く人は、「project の owner」と「workspace の owner」を
   別物として扱う必要がある。
6. **更新が必要なのは機能を使うサービスだけ。** 新機能を使うサービスは、accounts と通信する
   クライアントライブラリを更新する必要がある。更新しないサービスはそのまま動く。

技術的な詳細: *Solution Options → Option 1 → 技術的な補足: 影響・テスト・デプロイ・ロールバック・監視*
を参照。

## Test Plan

1. **基本ルール。** リソースの種別名と ID の検証と長さの上限。1ユーザーが1つの project に持てる
   ロールは1つ。ユーザーがその project の workspace から外れたら、project のロールは無視される。
2. **権限チェック。**
   - project を指定しないチェックは、今とまったく同じ結果になる。
   - 登録されていない project → 拒否。
   - project が属するものとは別の workspace を指定 → 拒否。
   - ユーザーの workspace ロールは、呼び出し側の指定ではなく、常に project が実際に属する
     workspace から取る。
   - project のロールがあれば適用され、なければ適用されない。
3. **project の登録。** 登録できるのはサービスだけ（ユーザーのログインでは拒否）。owner は
   workspace のメンバーでなければならない。同じ project を同じ workspace に再登録しても問題ないが、
   別の workspace への登録は拒否する。
4. **メンバーの管理。** workspace 外の人は追加できない。メンバーを管理する権限のない人は拒否される。
   メンバーは自分で抜けられる。最後の owner は外せず、降格もできない。
5. **後始末。** 1人または複数のユーザーを workspace から外したとき、workspace を削除したときに、
   project のロールが取り消される。
6. **同時の変更。** 2人が最後の2人の owner を同時に外す → ちょうど1つだけ成功する。2つのサービスが
   同じ project を同時に登録する → 1つだけ成功する。workspace の変更と project ロールの変更が同時に
   保存されても、project ロールは消えない。
7. **対応するすべてのデータベース。** 既存データに対するデータベースの更新とその取り消しを含む。
   既存ユーザーが project ロールなしで正しく読み込めること。
8. **エンドツーエンド。** Dashboard の project 用のサンプルポリシーで、project メンバー、非メンバー、
   workspace の owner、別 workspace のユーザーの結果を確認する。

技術的な詳細: *Solution Options → Option 1 → 技術的な補足: 影響・テスト・デプロイ・ロールバック・監視*
を参照。

## Deployment Plan

1. 後方互換性はあるか？ **あり。** すべて既存のものに追加する形。権限チェックで project を
   指定しないサービスには変化がない。
2. 段階的に展開できるか？ **できる。** 順序:
   1. accounts をデプロイする。
   2. 各サービスが既存の project を accounts に登録する（一度だけの埋め戻し）。
   3. 各サービスが、project の作成時に登録し、削除時に accounts へ知らせるようにする。
   4. 各サービスが project 単位の権限ルールを有効にし、権限チェックで project を指定し始める。
      このステップまでは project のロールは効かないため、誤ってアクセスできるようになる人はいない
      （安全側に倒れる）。
3. 通知先: dashboard / visualizer / cms のオーナー、権限ポリシーの管理者。
4. 設定変更: なし。
5. データベースの変更: accounts の起動時に自動で適用される。accounts 側で既存データを変換する
   必要はない。

技術的な詳細: *Solution Options → Option 1 → 技術的な補足: 影響・テスト・デプロイ・ロールバック・監視*
を参照。

## Rollback Plan

1. 原因をチームの Slack チャンネルに共有する。
2. まず、project 単位の権限ルールを使っているサービス側で、それを無効にする。
3. accounts を前のバージョンに戻す。前のバージョンは新しいデータを無視するだけなので、
   **データのロールバックは不要**。
   注意: 前のバージョンの稼働中は、権限データを保存したユーザーの project ロールが消えることがある。
   その時点では機能が無効なので許容でき、再度展開するときにサービスが埋め戻しをやり直す。
4. 新しいデータ構造を完全に取り除く必要がある場合のみ、データベースの取り消しを実行する。
   project の登録とロールの割り当てがすべて消えるので、先にバックアップを取る。

技術的な詳細: *Solution Options → Option 1 → 技術的な補足: 影響・テスト・デプロイ・ロールバック・監視*
を参照。

## Post Deployment

- チェックリスト
  - Dashboard の staging で project を作成し、登録されていることと、作成者が owner に
    なっていることを確認する。
  - workspace メンバーを project に招待し、アクセスできることを確認する。そのメンバーを
    workspace から外し、project にアクセスできなくなることを確認する。
  - 別の workspace を指定してその project への権限を問い合わせ、拒否されることを確認する。
- メトリクス
  - 権限チェックにかかる時間（通常時と、遅い方から 5%）。project を指定した場合と、しない場合。
  - project の登録 / 登録解除とメンバー変更のエラー率（自動リトライを含む）。
  - ユーザーあたりの project ロール数（最大と上位 1%）。1ユーザーあたり約 1,000 件という想定を監視する。
- アラート
  - 権限チェックのエラー率: Warning 1% 超、Danger 5% 以上。
  - 権限チェックの応答時間（遅い方から 5%）: 通常の基準より 20% 超遅くなったら Warning。

技術的な詳細: *Solution Options → Option 1 → 技術的な補足: 影響・テスト・デプロイ・ロールバック・監視*
を参照。

## Open Questions

1. **権限の判定に project の属性を使うか。** 「private な project はメンバーだけ、public な project は
   workspace 全体が見られる」のようなルールには、project が公開かどうかを知る必要がある。
   必要とするプロダクトが出てくるまで保留。
2. **一覧画面で複数の project をまとめてチェックできるようにするか。** project の一覧画面では
   project ごとに権限の判定が必要で、まとめて1回で判定できれば速くなる。
3. **各サービスの認証情報を、そのサービス自身のリソースに限定するか。** 最初のバージョンでは、
   すべてのサービスが共通のサービス用認証情報で project を登録する。各サービスが自分の種類の
   リソースだけを登録できるようにすべきか（例: Dashboard は Dashboard の project だけ）。
4. **サービスが知らせずに削除した project の登録を、自動で掃除する仕組みを作るか。** それとも、
   サービスが必ず知らせることに任せるか。
5. **integration を project のメンバーにする必要があるか**（例: cms で）。
6. **UI 上でどこでメンバーを管理するか。** project のメンバー管理を Dashboard だけで行うか、
   各サービス（visualizer / cms / flow）の画面でも行えるようにするか。accounts 側は変わらない
   （どの画面も同じメンバー管理の仕組みを使う）が、どのサービスに UI の対応が必要かが変わるため、
   デザイナーと合わせて決める。

技術的な詳細: *Solution Options → Option 1 → Open Questions の技術的な補足* を参照。

## Reviewed by

- Technical Architect: [name]
- Technical Leader: [name]
- Peers: [name]
