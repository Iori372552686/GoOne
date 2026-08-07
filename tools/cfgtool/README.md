# GoOne Game配置生成工具使用说明 (xlsx-trans)

## 工具功能

---
本工具用于将Excel/XLSX文件转换为多种格式的配置文件，主要用于游戏开发中的配置管理。它支持从Excel文件中读取数据，并生成对应的Protocol Buffers文本/二进制格式、JSON、Lua配置表，以及Go/C++/Node.js(TypeScript)多语言的配置加载与便捷查询代码。

---

### 主要功能

1. 支持对xlsx配置结构、内嵌结构体、枚举定义生成Proto文件
2. 支持将xlsx数据转换成pb bytes数据、pb text数据、JSON数据
3. 支持将xlsx直接的Lua配置代码
4. 支持xlsx表格内填入中文枚举、中文sheet名
5. 支持根据配置结构，生成对应的 **Go / C++ / Node.js(TypeScript)** 调用代码，支持生成多维key index查询map方法
6. 支持根据配置gen模式，生成不同内容的配置文件：
   - `all`：生成全部配置
   - `client`：仅生成客户端配置
   - `server`：仅生成服务器配置
7. 支持在xlsx第四行标记 `key` / `KEY`，自动生成主键Map索引，无需在生成表中手写 `map:字段名` 规则
8. 统一的错误日志输出，包含文件名、表名、字段名、行号、错误类型，方便快速定位问题

---

### 支持的输出格式

| 格式 | 说明 |
|------|------|
| Protocol Buffers 定义 (.proto) | 根据xlsx表结构自动生成proto文件 |
| Protocol Buffers 文本 (.conf) | pb text格式数据文件 |
| Protocol Buffers 二进制 (.bytes) | pb bytes格式数据文件 |
| JSON (.json) | JSON格式数据文件 |
| Lua (.lua) | Lua配置表 |
| Go 代码 (.gen.go) | Go语言配置加载与便捷查询代码 |
| C++17 代码 (.gen.hpp) | C++17配置加载与便捷查询头文件 |
| Node.js/TypeScript 代码 (.gen.ts) | TypeScript配置加载与便捷查询代码，带完整类型定义 |

---

## 安装与使用

### 安装Git
如果您的系统尚未安装Git，请先安装Git。可以从[Git官网](https://git-scm.com/downloads)下载并安装。

### 使用运行脚本 (bat / sh)

项目包含一个批处理脚本 `run_me.bat`，用于简化执行流程。编辑该脚本设置您的参数，然后直接运行：

```bash
xlsx_trans.exe ^
  -xlsx=./xls ^
  -text=./gen/text ^
  -proto=./gen/proto ^
  -json=./gen/json ^
  -bytes=./gen/bytes ^
  -lua=./gen/lua ^
  -ts=./gen/ts ^
  -code=./gen/code ^
  -cpp=./gen/cpp ^
  -nodejs=./gen/nodejs ^
  -mode=all
pause
```

> 所有输出目录参数为空则不生成对应格式，可按需开启。

---

## 工具参数说明

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-xlsx` | Excel文件目录 | `./xls` |
| `-text` | 生成pb text格式文件目录 | 空（不生成） |
| `-proto` | 生成proto定义文件目录 | 空（不生成） |
| `-json` | 生成JSON格式文件目录 | 空（不生成） |
| `-bytes` | 生成pb bytes格式文件目录 | 空（不生成） |
| `-lua` | 生成Lua配置表文件目录 | 空（不生成） |
| `-ts` | 生成TypeScript类型定义文件目录 | 空（不生成） |
| `-code` | 生成Go语言代码文件目录 | 空（不生成） |
| `-cpp` | 生成C++17代码文件目录 | 空（不生成） |
| `-nodejs` | 生成Node.js/TypeScript代码文件目录 | 空（不生成） |
| `-mode` | 配置生成模式（`all` / `client` / `server`） | `all` |
| `-module` | 生成代码导出项目目录 | `github.com/Iori372552686/GoOne` |
| `-pb` | Protocol Buffers生成路径 | `github.com/Iori372552686/game_protocol/protocol` |
| `-version` | 打印当前程序版本号 | - |

---

## 配置文件格式说明

### Excel表结构规范

```
第一行：中文列名，作为注解（如 道具类型、道具名称 等）
第二行：字段名，作为字段标识（如 Id、Name 等）
第三行：数据类型（如 int32、int64、[]uint32、string、[][]int64、[][][]int64 等）
第四行：配置gen模式（all、client、server、key）
第五行起：实际数据行
```

### 第四行标记说明

| 标记 | 说明 |
|------|------|
| `all` | 所有模式都生成该列 |
| `client` | 仅 `-mode=client` 时生成该列 |
| `server` | 仅 `-mode=server` 时生成该列 |
| `key` / `KEY` | 所有模式都生成（等同 all），**同时自动作为主键Map索引** |

`key` 标记可以标在一个或多个列上：
- **单列 key**：自动生成 `GetById(Id)` 风格的单字段索引
- **多列 key**：自动生成复合键索引（如 `GetByTypeLevel(Type, Level)`）

> **key 与 map 规则的关系**：`key` 标记是 xlsx 表内的列级声明，替代在生成表中写 `map:字段名` 的方式。两者可以共存——`key` 生成的索引与 `map` 规则生成的索引互不影响。

**示例：**

| 道具ID | 道具名称 | 品质 | 价格 |
|--------|---------|------|------|
| Id | Name | Quality | Price |
| int32 | string | int32 | int64 |
| **key** | all | all | server |

上面的配置中 `Id` 列标记为 `key`，工具将自动生成 `GetById(Id int32)` 的 Map 索引方法，无需在生成表中额外写 `map:Id`。

### 支持的数据类型

| 类型 | 说明 | 示例 |
|------|------|------|
| `int` / `int32` | 32位整数 | `100` |
| `int64` | 64位整数 | `100000` |
| `uint32` | 无符号32位整数 | `100` |
| `uint64` | 无符号64位整数 | `100000` |
| `float` | 32位浮点数 | `3.14` |
| `float64` / `double` | 64位浮点数 | `3.14159` |
| `string` | 字符串 | `hello` |
| `bool` | 布尔值 | `true` / `false` |
| `[]int32` | 整数数组 | `1,2,3` |
| `[]string` | 字符串数组 | `a,b,c` |
| `[][]int64` | 二维整数数组 | `1,2\|3,4` |
| `[][][]int64` | 三维整数数组 | `1,2\|3,4;5,6\|7,8` |
| `[][]string` | 二维字符串数组 | `a,b\|c,d` |
| `[][]float64` / `[][]double` | 二维浮点数组 | `1.1,2.2\|3.3` |
| `[][]Reward` | 二维结构体数组 | `1:10\|2:20;3:30\|4:40` |
| `[][][]Reward` | 三维结构体数组 | `1:10\|2:20;3:30\|4:40^5:50\|6:60;7:70\|8:80` |
| `map[int32]string` | 整数→字符串 映射 | `1:hp,2:mp,3:atk` |
| `map[string]int64` | 字符串→整数 映射 | `hp:100,mp:50` |
| `map[int32]Reward` | 整数→结构体 映射 | `1:1:10\|2:2:20` |
| 枚举名 | 枚举类型，可中文 | `金币` |
| 结构名 | 引用结构体 | `1:100:name` |

### 多维数组说明

当前实现**最多支持 3 维数组**。底层 protobuf 方案是递归包装 message，因此**理论上**可以继续扩展到 4 维、5 维；但如果要正式开启更高维，需要同步扩展代码中的维度上限、分隔符规则与测试用例。

#### 基础类型 / 枚举数组分隔符

| 维度 | 分隔符 | 示例类型 | 示例值 |
|------|--------|----------|--------|
| 1维 | `,` | `[]int64` | `1,2,3` |
| 2维 | `\|` | `[][]int64` | `1,2\|3,4` |
| 3维 | `;` | `[][][]int64` | `1,2\|3,4;5,6\|7,8` |

#### 结构体数组分隔符

结构体单个元素内部继续使用 `:` 表示字段顺序，因此数组层级会避开 `,`，采用另一组分隔符：

| 维度 | 分隔符 | 示例类型 | 示例值 |
|------|--------|----------|--------|
| 1维 | `\|` | `[]Reward` | `1:10\|2:20\|3:30` |
| 2维 | `;` | `[][]Reward` | `1:10\|2:20;3:30\|4:40` |
| 3维 | `^` | `[][][]Reward` | `1:10\|2:20;3:30\|4:40^5:50\|6:60;7:70\|8:80` |

### map 类型说明

工具支持 `map[K]V` 字段类型（Go 风格语法），对应 protobuf3 原生 `map<K, V>`，用于表达键值映射配置。

**语法：** 在 xlsx 第三行（数据类型行）写 `map[K]V`，如 `map[int32]string`、`map[string]int64`、`map[int32]Reward`。

**K（键）限制** —— 受 protobuf3 规范约束：

| 允许的 K | 说明 |
|----------|------|
| `int32` / `int64` / `uint32` / `uint64` | 整数键 |
| `string` | 字符串键 |
| `bool` | 布尔键 |

> 浮点（`float`/`double`）、枚举、结构体、数组**不能**作为 map 的 key。

**V（值）支持：**

| V 类型 | 说明 | 示例类型 |
|-------|------|---------|
| 标量 | `int32`/`int64`/`string`/`bool` 等 | `map[int32]string` |
| 枚举 | 引用枚举名 | `map[int32]PropertyType` |
| 结构体 | 引用 `@struct` 定义名 | `map[int32]Reward` |

> 本期**不支持** `map[K][]V`（值为数组）与 `map[K]map[K2]V`（嵌套 map）。如需更高复杂度，建议拆分为结构体数组或单独配置表。

#### map 元素分隔符

每个 map 元素用 `K:V` 表示，元素之间按 **value 类型** 选择分隔符（与结构体数组规则一致，避开结构体内部的 `:`）：

| value 类型 | 元素分隔符 | 示例 |
|-----------|-----------|------|
| 标量 / 枚举 | `,` | `1:hp,2:mp,3:atk` |
| 结构体 | `\|` | `1:1:10\|2:2:20` |

> 解析时按第一个 `:` 切分 K 与 V，因此**结构体 value 内部的 `:` 不受影响**。

#### map 字段示例

假设结构体 `Reward` 定义为 `ItemId:Count`：

| 编号 | 属性名映射 | 标签数值 | ID奖励 |
|------|-----------|---------|--------|
| Id | Attrs | Tags | Rewards |
| `int32` | `map[int32]string` | `map[string]int64` | `map[int32]Reward` |
| key | all | all | all |
| 1 | `1:hp,2:mp,3:atk` | `hp:100,mp:50,speed:20` | `1:1:10\|2:2:20` |

对应的 proto 输出：
```proto
message MapConfig {
    int32 Id = 1;
    map<int32, string> Attrs = 2;
    map<string, int64> Tags = 3;
    map<int32, Reward> Rewards = 4;
}
```

对应的 JSON 输出（map 字段渲染为对象）：
```json
{
  "Id": 1,
  "Attrs": { "1": "hp", "2": "mp", "3": "atk" },
  "Tags": { "hp": 100, "mp": 50, "speed": 20 },
  "Rewards": {
    "1": { "ItemId": 1, "Count": 10 },
    "2": { "ItemId": 2, "Count": 20 }
  }
}
```

#### 三维数组示例

**基础类型三维数组：**

| 奖励立方体 |
|------------|
| RewardCube |
| `[][][]int64` |
| all |
| `1,2\|3,4;5,6\|7,8` |

上面的值会被解析为：

```json
[
  [
    [1, 2],
    [3, 4]
  ],
  [
    [5, 6],
    [7, 8]
  ]
]
```

**结构体三维数组：**

假设结构体 `Reward` 定义为 `ItemId:Count`，那么：

| 三维奖励 |
|----------|
| RewardCube |
| `[][][]Reward` |
| all |
| `1:10\|2:20;3:30\|4:40^5:50\|6:60;7:70\|8:80` |

会被解析为 2 个二维面，每个二维面包含 2 行结构体数组。

### 枚举类型说明

```
E|道具类型-金币|PropertType|Coin|1
```

格式：`E|中文描述|枚举类型名|枚举值名|枚举值`

### 配置规则说明（生成表中填写）

```
@config|sheet:结构名|map:字段名[,字段名]:别名|lua:参数1,参数2
@struct|sheet:结构名
@enum|sheet
```

- `@config`：定义配置表，支持 `map` 和 `group` 索引
- `@struct`：定义结构体表
- `@enum`：定义枚举表

---

## 各语言生成代码说明

### 统一 API 一览

三种语言生成代码遵循**统一命名风格**，方法名与语义完全对齐：

| 方法 | 说明 | Go | C++ | TypeScript |
|------|------|:--:|:---:|:----------:|
| `GetHead` | 获取第一条记录 | `GetHead()` | `GetHead()` | `GetHead()` |
| `GetAll` | 获取全部记录（拷贝） | `GetAll()` | `GetAll()` | `GetAll()` |
| `Count` | 获取记录总数 | `Count()` | `Count()` | `Count()` |
| `Range` | 遍历，回调返回 false 终止 | `Range(fn)` | `Range(fn)` | `Range(fn)` |
| `Find` | 条件查找，返回第一个匹配项 | `Find(fn)` | `Find(fn)` | `Find(fn)` |
| `Filter` | 条件过滤，返回所有匹配项 | `Filter(fn)` | `Filter(fn)` | `Filter(fn)` |
| `GetByXxx` | 按 Map 索引精确查找 | `GetByXxx(key)` | `GetByXxx(key)` | `GetByXxx(key)` |
| `GroupByXxx` | 按 Group 索引分组查找 | `GroupByXxx(key)` | `GroupByXxx(key)` | `GroupByXxx(key)` |

> `Xxx` 由 xlsx 中 `map:字段名:别名` 规则决定，自动生成。

---

### Go 代码 (`-code`)

为每个配置生成独立的 `.gen.go` 文件，包含：
- 配置数据结构体 + `init()` 自动注册
- 基础查询：`GetHead()` / `GetAll()` / `Count()`
- 遍历：`Range(fn)`
- 条件查询：`Find(fn)` / `Filter(fn)`
- 索引查询：`GetByXxx(key)` / `GroupByXxx(key)`

### C++17 代码 (`-cpp`)

为每个配置生成独立的 `.gen.hpp` 头文件，包含：
- 单例数据管理类，`ParseFromText()` 加载数据
- 基础查询：`GetHead()` / `GetAll()` / `Count()`
- 遍历：`Range(fn)`
- 条件查询：`Find(fn)` / `Filter(fn)`
- 索引查询：`GetByXxx(key)` / `GroupByXxx(key)`

### Node.js/TypeScript 代码 (`-nodejs`)

为每个配置生成独立的 `.gen.ts` 文件，包含：
- 完整 TypeScript 接口定义（`IXxxConfig`），含引用的子结构体接口
- 单例数据管理器类：
  - 加载：`Parse(data)`
  - 基础查询：`GetHead()` / `GetAll()` / `Count()`
  - 遍历：`Range(fn)`
  - 条件查询：`Find(fn)` / `Filter(fn)`
  - 索引查询：`GetByXxx(key)` / `GroupByXxx(key)`
- 自动生成 `index.ts` 入口文件，统一导出所有配置

---

### 使用示例

**Go：**
```go
item := item_config.GetByList(1001)         // Map 索引查找
all  := item_config.GetAll()                // 获取全部
cnt  := item_config.Count()                 // 记录数
rare := item_config.Filter(func(c *pb.ItemConfig) bool {
    return c.Quality >= 4                   // 条件过滤
})
first := item_config.Find(func(c *pb.ItemConfig) bool {
    return c.Name == "金币"                  // 条件查找
})
```

**C++17：**
```cpp
auto* item = ItemData::GetByList(1001);      // Map 索引查找
auto  all  = ItemData::GetAll();             // 获取全部
auto  cnt  = ItemData::Count();              // 记录数
auto  rare = ItemData::Filter([](const auto* c) {
    return c->quality() >= 4;                // 条件过滤
});
auto* first = ItemData::Find([](const auto* c) {
    return c->name() == "金币";               // 条件查找
});
```

**TypeScript / Node.js：**
```typescript
import { item } from './gen/nodejs';
import * as fs from 'fs';

// 加载
item.Parse(JSON.parse(fs.readFileSync('./gen/json/ItemConfig.json', 'utf-8')));

// 使用
const head = item.GetHead();                 // 第一条
const all  = item.GetAll();                  // 全部
const cnt  = item.Count();                   // 记录数
const it   = item.GetByList(1001);           // Map 索引查找
const rare = item.Filter(c => c.Quality >= 4);  // 条件过滤
const first = item.Find(c => c.Name === '金币'); // 条件查找
item.Range(c => { console.log(c.Name); return true; }); // 遍历
```

---

## 注意事项

- 路径处理：支持Windows路径（反斜杠）和Linux路径（正斜杠），相对路径基于执行目录
- 所有输出目录参数留空则不生成对应格式，按需开启即可
- 结构体字段按定义顺序赋值，无需额外标记
- 错误日志统一输出包含：文件名、表名、字段名、行号、错误类型，便于快速定位
- Node.js代码依赖JSON输出，建议同时开启 `-json` 和 `-nodejs`
- C++代码依赖proto生成的 `.pb.h`，请确保protoc的C++输出路径正确
