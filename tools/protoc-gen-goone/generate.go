package main

import (
	"errors"
	"fmt"
	"go/format"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/pluginpb"
)

const (
	defaultModulePath              = "github.com/Iori372552686/GoOne"
	ssrpcOptFilePath               = "goone/options/v1/options.proto"
	ssrpcExtFullName               = "goone.options.v1.ssrpc"
	ssrpcServiceExtFullName        = "goone.options.v1.ssrpc_service"
	gameProtocolPath               = "github.com/Iori372552686/game_protocol/protocol"
	defaultSSRPCTimeoutMs   uint32 = 5000
)

// Generate is the Phase-A generator:
//   - It DOES NOT generate pb.go (handled by protoc-gen-go).
//   - It generates SSPacket RPC registration code (*goone_ssrpc.gen.go) for methods
//     annotated with option (goone.options.v1.ssrpc).
func Generate(req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
	files := map[string]*descriptorpb.FileDescriptorProto{}
	for _, f := range req.GetProtoFile() {
		files[f.GetName()] = f
	}

	extType, extMsgDesc, extNum, err := buildSsRpcExtension(req.GetProtoFile())
	if err != nil {
		return nil, err
	}
	serviceExtType, serviceExtMsgDesc, serviceExtNum, err := buildSsRpcServiceExtension(req.GetProtoFile())
	if err != nil {
		return nil, err
	}

	typeReg, err := BuildTypeRegistry(req.GetProtoFile())
	if err != nil {
		return nil, err
	}

	resp := new(pluginpb.CodeGeneratorResponse)
	resp.SupportedFeatures = proto.Uint64(uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL))

	params := parseParams(req.GetParameter())

	for _, name := range req.GetFileToGenerate() {
		fd := files[name]
		if fd == nil {
			continue
		}

		// Only generate for files that have services.
		if len(fd.GetService()) == 0 {
			continue
		}

		// Only generate if at least one method in this file has ssrpc option.
		has := false
		for _, s := range fd.GetService() {
			for _, m := range s.GetMethod() {
				_, ok, err := readSsRpcOption(m.GetOptions(), extType, extMsgDesc, extNum)
				if err != nil {
					return nil, err
				}
				if ok {
					has = true
					break
				}
			}
			if has {
				break
			}
		}
		if !has {
			continue
		}

		outName, pkgName, err := outputFileNameAndPkg(fd, name, params)
		if err != nil {
			return nil, err
		}

		curGoPkgOpt := fd.GetOptions().GetGoPackage()
		curImportPath, _ := splitGoPackage(curGoPkgOpt, fd.GetPackage())

		content, err := renderSSRPC(fd, pkgName, curImportPath, typeReg, extType, extMsgDesc, extNum, serviceExtType, serviceExtMsgDesc, serviceExtNum)
		if err != nil {
			return nil, err
		}
		// 生成代码写入前执行 go/format，保证二次生成无 diff，
		// 避免手工拼接的缩进/空白飘动引入无语义变更。
		if formatted, fmtErr := format.Source([]byte(content)); fmtErr == nil {
			content = string(formatted)
		}
		resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
			Name:    &outName,
			Content: &content,
		})
	}
	return resp, nil
}

type genParams struct {
	paths  string // "import" (default) or "source_relative"
	module string
}

func parseParams(p string) genParams {
	gp := genParams{paths: "import", module: defaultModulePath}
	// protoc passes plugin parameter as comma-separated list
	for _, part := range strings.Split(p, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "paths=") {
			gp.paths = strings.TrimPrefix(part, "paths=")
		}
		if strings.HasPrefix(part, "module=") {
			gp.module = strings.TrimPrefix(part, "module=")
		}
	}
	return gp
}

func outputFileNameAndPkg(fd *descriptorpb.FileDescriptorProto, protoPath string, p genParams) (outName string, pkgName string, err error) {
	pkg := fd.GetPackage()
	goPkg := fd.GetOptions().GetGoPackage()
	pkgName = guessGoPkgName(goPkg, pkg)

	base := strings.TrimSuffix(filepath.Base(protoPath), filepath.Ext(protoPath))

	if p.paths == "source_relative" || goPkg == "" {
		// source_relative output next to proto file path
		outName = strings.TrimSuffix(protoPath, filepath.Ext(protoPath)) + ".goone_ssrpc.gen.go"
		return outName, pkgName, nil
	}

	// import-based output: honor go_package.
	goPkgPath := goPkg
	if i := strings.LastIndex(goPkgPath, ";"); i >= 0 {
		goPkgPath = goPkgPath[:i]
	}
	modulePrefix := strings.TrimRight(p.module, "/") + "/"
	if !strings.HasPrefix(goPkgPath, modulePrefix) {
		return "", "", fmt.Errorf("go_package must start with %q, got %q (file=%s)", modulePrefix, goPkgPath, protoPath)
	}
	relDir := strings.TrimPrefix(goPkgPath, modulePrefix)
	outName = path.Join(relDir, base+".goone_ssrpc.gen.go")
	return outName, pkgName, nil
}

func renderSSRPC(fd *descriptorpb.FileDescriptorProto, goPkgName string, curImportPath string, typeReg *TypeRegistry, extType protoreflect.ExtensionType, extMsgDesc protoreflect.MessageDescriptor, extNum protowire.Number, serviceExtType protoreflect.ExtensionType, serviceExtMsgDesc protoreflect.MessageDescriptor, serviceExtNum protowire.Number) (string, error) {
	var b strings.Builder
	b.WriteString("// Code generated by protoc-gen-goone. DO NOT EDIT.\n\n")
	b.WriteString("package " + goPkgName + "\n\n")

	// We collect imports dynamically to support cross-package message types.
	ib := newImportBuilder()
	// reserve aliases used in file
	ib.usedAliases["g1_protocol"] = true
	ib.usedAliases["cmd_handler"] = true
	ib.usedAliases["ssrpc"] = true
	ib.usedAliases["transaction"] = true
	// Fix the alias for game protocol to avoid generating a duplicate import (g1_protocol2)
	// when message types also come from the same module.
	ib.byPath[gameProtocolPath] = "g1_protocol"

	// generate per service
	localPkg := fd.GetPackage()

	type methodInfo struct {
		name             string
		inGo             string
		outGo            string
		grpcServerStream bool
	}

	type httpBind struct {
		method      string
		inGo        string
		outGo       string
		path        string
		httpMethod  string
		grpcService string

		cmdLit string // literal for MethodDesc.Cmd, can be "0" for non-cmd transports

		grpcServerStream bool

		oneWay    bool
		uidLock   bool
		auth      bool
		sign      bool
		timeoutMs uint32
		tags      []string
		name      string
	}

	typeRef := func(fullType string) (string, string, error) {
		// If using source_relative, allow local package type names even without go_package.
		// This keeps behavior consistent with protoc-gen-go default package resolution.
		if strings.HasPrefix(strings.TrimPrefix(fullType, "."), localPkg+".") {
			localType, err := goTypeNameFromProtoType(localPkg, fullType)
			if err == nil {
				return localType, "", nil
			}
		}

		gt, ok := typeReg.Resolve(fullType)
		if !ok {
			return "", "", fmt.Errorf("cannot resolve proto type %q (did you set go_package for that file?)", fullType)
		}
		if gt.ImportPath == "" || gt.PkgName == "" {
			return "", "", fmt.Errorf("invalid go_package for proto type %q (importPath=%q pkgName=%q)", fullType, gt.ImportPath, gt.PkgName)
		}
		if curImportPath != "" && gt.ImportPath == curImportPath {
			return gt.TypeName, "", nil
		}
		if gt.ImportPath == gameProtocolPath {
			return "g1_protocol." + gt.TypeName, gt.ImportPath, nil
		}
		alias := ib.add(gt.ImportPath, gt.PkgName)
		return alias + "." + gt.TypeName, gt.ImportPath, nil
	}

	// collect used message imports (excluding current package)
	usedMsgImports := map[string]struct{}{} // importPath -> exists
	for _, s := range fd.GetService() {
		svc := s.GetName()
		if svc == "" {
			continue
		}

		serviceOpt, _, err := readSsRpcServiceOption(s.GetOptions(), serviceExtType, serviceExtMsgDesc, serviceExtNum)
		if err != nil {
			return "", err
		}
		serviceTimeoutMs := serviceOpt.timeoutMs

		b.WriteString(fmt.Sprintf("// %sSS is the ssrpc service interface for %s.\n", svc, svc))
		b.WriteString(fmt.Sprintf("type %sSS interface {\n", svc))
		// collect methods that are annotated with ssrpc option.
		annotated := make([]*descriptorpb.MethodDescriptorProto, 0, len(s.GetMethod()))
		for _, m := range s.GetMethod() {
			opts := m.GetOptions()
			_, ok, err := readSsRpcOption(opts, extType, extMsgDesc, extNum)
			if err != nil {
				return "", err
			}
			if ok {
				annotated = append(annotated, m)
			}
		}

		methods := make([]methodInfo, 0, len(annotated))
		for _, m := range annotated {
			method := m.GetName()
			if method == "" {
				continue
			}
			ext, _, err := readSsRpcOption(m.GetOptions(), extType, extMsgDesc, extNum)
			if err != nil {
				return "", err
			}
			hasCmd := !(ext.cmd == 0 && ext.cmdEnum == 0 && strings.TrimSpace(ext.cmdName) == "")
			hasHTTP := strings.TrimSpace(ext.httpPath) != ""
			if m.GetClientStreaming() {
				return "", fmt.Errorf("ssrpc only supports unary or gRPC server-streaming methods today (service=%s method=%s)", svc, method)
			}
			grpcServerStream := ext.grpc && m.GetServerStreaming()
			if m.GetServerStreaming() {
				if !ext.grpc {
					return "", fmt.Errorf("ssrpc server-streaming method requires grpc=true (service=%s method=%s)", svc, method)
				}
				if hasCmd || hasHTTP || ext.ws {
					return "", fmt.Errorf("ssrpc server-streaming method must be grpc-only today (service=%s method=%s)", svc, method)
				}
			}
			inGo, imp1, err := typeRef(m.GetInputType())
			if err != nil {
				return "", err
			}
			if imp1 != "" {
				usedMsgImports[imp1] = struct{}{}
			}
			outGo, imp2, err := typeRef(m.GetOutputType())
			if err != nil {
				return "", err
			}
			if imp2 != "" {
				usedMsgImports[imp2] = struct{}{}
			}
			methods = append(methods, methodInfo{
				name:             method,
				inGo:             inGo,
				outGo:            outGo,
				grpcServerStream: grpcServerStream,
			})
			if grpcServerStream {
				b.WriteString(fmt.Sprintf("\t%s(ctx *ssrpc.Context, req *%s, stream *ssrpc.ServerStream[*%s]) error\n", method, inGo, outGo))
				continue
			}
			b.WriteString(fmt.Sprintf("\t%s(ctx *ssrpc.Context, req *%s) (*%s, error)\n", method, inGo, outGo))
		}
		b.WriteString("}\n\n")

		// Optional scaffold: provide an Unimplemented<Service>SS implementation that compiles.
		b.WriteString(fmt.Sprintf("// Unimplemented%sSS can be embedded/used to have forward compatible implementations.\n", svc))
		b.WriteString(fmt.Sprintf("type Unimplemented%sSS struct{}\n\n", svc))
		b.WriteString(fmt.Sprintf("var _ %sSS = (*Unimplemented%sSS)(nil)\n\n", svc, svc))
		for _, mi := range methods {
			if mi.grpcServerStream {
				b.WriteString(fmt.Sprintf("func (*Unimplemented%sSS) %s(ctx *ssrpc.Context, req *%s, stream *ssrpc.ServerStream[*%s]) error {\n", svc, mi.name, mi.inGo, mi.outGo))
				b.WriteString(fmt.Sprintf("\treturn ssrpc.Unimplemented(%q)\n", svc+"."+mi.name))
				b.WriteString("}\n\n")
				continue
			}
			b.WriteString(fmt.Sprintf("func (*Unimplemented%sSS) %s(ctx *ssrpc.Context, req *%s) (*%s, error) {\n", svc, mi.name, mi.inGo, mi.outGo))
			b.WriteString(fmt.Sprintf("\treturn nil, ssrpc.Unimplemented(%q)\n", svc+"."+mi.name))
			b.WriteString("}\n\n")
		}

		// Optional layered runtime helpers: default middleware builder + server constructor.
		b.WriteString(fmt.Sprintf("// Default%sSSMiddlewares returns the standard middleware chain for %s.\n", svc, svc))
		b.WriteString(fmt.Sprintf("func Default%sSSMiddlewares(opts ssrpc.DefaultMWOptions) []ssrpc.Middleware {\n", svc))
		b.WriteString("\treturn ssrpc.DefaultMiddlewares(opts)\n")
		b.WriteString("}\n\n")
		b.WriteString(fmt.Sprintf("// New%sSServer constructs a %sSServer with a default middleware chain.\n", svc, svc))
		b.WriteString(fmt.Sprintf("func New%sSServer(impl %sSS, opts ssrpc.DefaultMWOptions) %sSServer {\n", svc, svc, svc))
		b.WriteString(fmt.Sprintf("\treturn %sSServer{Impl: impl, MW: ssrpc.DefaultMiddlewares(opts)}\n", svc))
		b.WriteString("}\n\n")

		b.WriteString(fmt.Sprintf("type %sSServer struct {\n", svc))
		b.WriteString(fmt.Sprintf("\tImpl %sSS\n", svc))
		b.WriteString("\tMW []ssrpc.Middleware\n")
		b.WriteString("}\n\n")

		httpBinds := make([]httpBind, 0)
		wsBinds := make([]httpBind, 0)   // reuse httpBind struct; ws methods only need cmd+method+desc
		grpcBinds := make([]httpBind, 0) // reuse httpBind struct for grpc methods
		hasAnyCmdMethod := false
		hasAnyWSMethod := false
		hasAnyGRPCMethod := false
		for _, m := range annotated {
			ext, _, err := readSsRpcOption(m.GetOptions(), extType, extMsgDesc, extNum)
			if err != nil {
				return "", err
			}
			timeoutMs := effectiveTimeoutMs(ext.timeoutMs, serviceTimeoutMs)
			hasCmd := !(ext.cmd == 0 && ext.cmdEnum == 0 && strings.TrimSpace(ext.cmdName) == "")
			hasHTTP := strings.TrimSpace(ext.httpPath) != ""
			hasWS := ext.ws && hasCmd // ws requires a cmd binding
			hasGRPC := ext.grpc
			if m.GetClientStreaming() {
				return "", fmt.Errorf("ssrpc only supports unary or gRPC server-streaming methods today (service=%s method=%s)", svc, m.GetName())
			}
			if m.GetServerStreaming() {
				if !ext.grpc {
					return "", fmt.Errorf("ssrpc server-streaming method requires grpc=true (service=%s method=%s)", svc, m.GetName())
				}
				if hasCmd || hasHTTP || ext.ws {
					return "", fmt.Errorf("ssrpc server-streaming method must be grpc-only today (service=%s method=%s)", svc, m.GetName())
				}
			}
			if !hasCmd && !hasHTTP && !hasGRPC {
				return "", fmt.Errorf("ssrpc option requires cmd/cmd_enum/cmd_name OR http_path OR grpc=true (service=%s method=%s)", svc, m.GetName())
			}
			if hasCmd {
				hasAnyCmdMethod = true
			}
			if hasWS {
				hasAnyWSMethod = true
			}
			if hasGRPC {
				hasAnyGRPCMethod = true
			}

			// cmd expression (prefer explicit numeric cmd; otherwise use cmd_name -> g1_protocol.<NAME>)
			cmdExpr := ""
			if hasCmd {
				if ext.cmd != 0 {
					cmdExpr = fmt.Sprintf("g1_protocol.CMD(0x%X)", ext.cmd)
				} else if ext.cmdEnum != 0 {
					cmdExpr = fmt.Sprintf("g1_protocol.CMD(0x%X)", uint32(ext.cmdEnum))
				} else {
					cmdExpr = "g1_protocol." + strings.TrimSpace(ext.cmdName)
				}
			}
			cmdLit := "0"
			if hasCmd {
				cmdLit = cmdExpr
			}

			// cmd_resp expression
			cmdRespExpr := ""
			if hasCmd {
				if ext.cmdResp != 0 {
					cmdRespExpr = fmt.Sprintf("g1_protocol.CMD(0x%X)", ext.cmdResp)
				} else {
					// default resp = cmd + 1
					cmdRespExpr = fmt.Sprintf("g1_protocol.CMD(uint32(%s) + 1)", cmdExpr)
				}
			}

			oneWay := ext.oneWay
			inGo, imp1, err := typeRef(m.GetInputType())
			if err != nil {
				return "", err
			}
			if imp1 != "" {
				usedMsgImports[imp1] = struct{}{}
			}
			outGo, imp2, err := typeRef(m.GetOutputType())
			if err != nil {
				return "", err
			}
			if imp2 != "" {
				usedMsgImports[imp2] = struct{}{}
			}

			method := m.GetName()

			// Phase 2: collect gin HTTP bindings.
			if p := strings.TrimSpace(ext.httpPath); p != "" {
				hm := strings.ToUpper(strings.TrimSpace(ext.httpMethod))
				if hm == "" {
					hm = "POST"
				}
				name := ext.comment
				if name == "" {
					name = svc + "." + method
				}
				httpBinds = append(httpBinds, httpBind{
					method:     method,
					inGo:       inGo,
					outGo:      "",
					path:       p,
					httpMethod: hm,
					cmdLit:     cmdLit,
					oneWay:     oneWay,
					uidLock:    ext.uidLock,
					auth:       ext.auth,
					sign:       ext.sign,
					timeoutMs:  timeoutMs,
					tags:       ext.tags,
					name:       name,
				})
			}

			// Phase 2: collect WS (CSPacket) bindings for methods with ws=true.
			if hasWS {
				name := ext.comment
				if name == "" {
					name = svc + "." + method
				}
				wsBinds = append(wsBinds, httpBind{
					method:    method,
					inGo:      inGo,
					outGo:     "",
					cmdLit:    cmdExpr,
					oneWay:    oneWay,
					uidLock:   ext.uidLock,
					auth:      ext.auth,
					sign:      ext.sign,
					timeoutMs: timeoutMs,
					tags:      ext.tags,
					name:      name,
				})
			}

			// Phase 3: collect gRPC bindings for methods with grpc=true.
			if hasGRPC {
				name := ext.comment
				if name == "" {
					name = svc + "." + method
				}
				grpcServiceName := strings.TrimSpace(ext.grpcService)
				if grpcServiceName == "" {
					grpcServiceName = svc
					if pkg := strings.TrimSpace(fd.GetPackage()); pkg != "" {
						grpcServiceName = pkg + "." + svc
					}
				}
				grpcBinds = append(grpcBinds, httpBind{
					method:           method,
					inGo:             inGo,
					outGo:            outGo,
					grpcService:      grpcServiceName,
					cmdLit:           cmdLit,
					grpcServerStream: m.GetServerStreaming(),
					oneWay:           oneWay,
					uidLock:          ext.uidLock,
					auth:             ext.auth,
					sign:             ext.sign,
					timeoutMs:        timeoutMs,
					tags:             ext.tags,
					name:             name,
				})
			}

			if !hasCmd {
				continue
			}

			if !strings.Contains(b.String(), fmt.Sprintf("func Register%sToTransactionMgr", svc)) {
				// emit header lazily when first cmd-bound method encountered
				// (keeps HTTP-only services from importing transaction/g1_protocol).
				b.WriteString(fmt.Sprintf("// Register%sToTransactionMgr binds SSPacket cmd -> handler wrappers.\n", svc))
				b.WriteString(fmt.Sprintf("func Register%sToTransactionMgr(mgr transaction.ITransactionMgr, srv %sSServer) {\n", svc, svc))
				b.WriteString("\tif mgr == nil || srv.Impl == nil {\n")
				b.WriteString("\t\treturn\n")
				b.WriteString("\t}\n\n")
			}

			b.WriteString(fmt.Sprintf("\tmgr.RegisterCmd(%s, ssrpc.WrapUnary(\n", cmdExpr))
			b.WriteString("\t\tssrpc.MethodDesc{\n")
			b.WriteString(fmt.Sprintf("\t\t\tCmd: %s,\n", cmdExpr))
			// If cmd_resp not explicitly specified, keep CmdResp=0 to use default cmd+1 in runtime.
			if ext.cmdResp != 0 {
				b.WriteString(fmt.Sprintf("\t\t\tCmdResp: %s,\n", cmdRespExpr))
			}
			if oneWay {
				b.WriteString("\t\t\tOneWay: true,\n")
			}
			if ext.uidLock {
				b.WriteString("\t\t\tUIDLock: true,\n")
			}
			if ext.auth {
				b.WriteString("\t\t\tAuth: true,\n")
			}
			if ext.sign {
				b.WriteString("\t\t\tSign: true,\n")
			}
			writeTimeoutField(&b, timeoutMs)
			if tagMap := parseTraceTags(ext.tags); len(tagMap) > 0 {
				b.WriteString("\t\t\tTraceTags: map[string]string{")
				keys := make([]string, 0, len(tagMap))
				for k := range tagMap {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					b.WriteString(fmt.Sprintf("%q: %q, ", k, tagMap[k]))
				}
				b.WriteString("},\n")
			}
			// Prefer proto option comment; otherwise use "Service.Method".
			if ext.comment != "" {
				b.WriteString(fmt.Sprintf("\t\t\tName: %q,\n", ext.comment))
			} else {
				b.WriteString(fmt.Sprintf("\t\t\tName: %q,\n", svc+"."+method))
			}
			b.WriteString("\t\t},\n")
			b.WriteString("\t\tsrv.MW,\n")
			b.WriteString(fmt.Sprintf("\t\tfunc() any { return new(%s) },\n", inGo))
			b.WriteString("\t\tfunc(ctx *ssrpc.Context, in any) (any, error) {\n")
			b.WriteString(fmt.Sprintf("\t\t\treturn srv.Impl.%s(ctx, in.(*%s))\n", method, inGo))
			b.WriteString("\t\t},\n")
			b.WriteString("\t))\n\n")
		}

		if hasAnyCmdMethod {
			b.WriteString("}\n\n")
		}

		// Optional Phase-2 output: gin route registration for methods with http_path.
		if len(httpBinds) > 0 {
			b.WriteString(fmt.Sprintf("// Register%sToGin binds HTTP routes -> service methods (Gin).\n", svc))
			b.WriteString(fmt.Sprintf("func Register%sToGin(r gin.IRoutes, srv %sSServer) {\n", svc, svc))
			b.WriteString("\tif r == nil || srv.Impl == nil {\n")
			b.WriteString("\t\treturn\n")
			b.WriteString("\t}\n\n")
			for _, hb := range httpBinds {
				b.WriteString(fmt.Sprintf("\tr.Handle(%q, %q, ssrpc.WrapHTTPGin(\n", hb.httpMethod, hb.path))
				b.WriteString("\t\tssrpc.MethodDesc{\n")
				b.WriteString(fmt.Sprintf("\t\t\tCmd: %s,\n", hb.cmdLit))
				if hb.oneWay {
					b.WriteString("\t\t\tOneWay: true,\n")
				}
				if hb.uidLock {
					b.WriteString("\t\t\tUIDLock: true,\n")
				}
				if hb.auth {
					b.WriteString("\t\t\tAuth: true,\n")
				}
				if hb.sign {
					b.WriteString("\t\t\tSign: true,\n")
				}
				writeTimeoutField(&b, hb.timeoutMs)
				if tagMap := parseTraceTags(hb.tags); len(tagMap) > 0 {
					b.WriteString("\t\t\tTraceTags: map[string]string{")
					keys := make([]string, 0, len(tagMap))
					for k := range tagMap {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						b.WriteString(fmt.Sprintf("%q: %q, ", k, tagMap[k]))
					}
					b.WriteString("},\n")
				}
				b.WriteString(fmt.Sprintf("\t\t\tName: %q,\n", hb.name))
				b.WriteString("\t\t},\n")
				b.WriteString("\t\tsrv.MW,\n")
				b.WriteString(fmt.Sprintf("\t\tfunc() any { return new(%s) },\n", hb.inGo))
				b.WriteString("\t\tfunc(ctx *ssrpc.Context, in any) (any, error) {\n")
				b.WriteString(fmt.Sprintf("\t\t\treturn srv.Impl.%s(ctx, in.(*%s))\n", hb.method, hb.inGo))
				b.WriteString("\t\t},\n")
				b.WriteString("\t))\n\n")
			}
			b.WriteString("}\n\n")
		}

		// Phase-2 output: WS (CSPacket) registration for methods with ws=true.
		if len(wsBinds) > 0 {
			b.WriteString(fmt.Sprintf("// Register%sToWS registers WS (CSPacket) cmd -> handler wrappers.\n", svc))
			b.WriteString(fmt.Sprintf("func Register%sToWS(d *ssrpc.Dispatcher, srv %sSServer) {\n", svc, svc))
			b.WriteString("\tif d == nil || srv.Impl == nil {\n")
			b.WriteString("\t\treturn\n")
			b.WriteString("\t}\n\n")
			for _, wb := range wsBinds {
				b.WriteString(fmt.Sprintf("\td.RegisterWS(uint32(%s), ssrpc.WrapWS(\n", wb.cmdLit))
				b.WriteString("\t\tssrpc.MethodDesc{\n")
				b.WriteString(fmt.Sprintf("\t\t\tCmd: %s,\n", wb.cmdLit))
				if wb.oneWay {
					b.WriteString("\t\t\tOneWay: true,\n")
				}
				if wb.uidLock {
					b.WriteString("\t\t\tUIDLock: true,\n")
				}
				if wb.auth {
					b.WriteString("\t\t\tAuth: true,\n")
				}
				if wb.sign {
					b.WriteString("\t\t\tSign: true,\n")
				}
				writeTimeoutField(&b, wb.timeoutMs)
				if tagMap := parseTraceTags(wb.tags); len(tagMap) > 0 {
					b.WriteString("\t\t\tTraceTags: map[string]string{")
					keys := make([]string, 0, len(tagMap))
					for k := range tagMap {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						b.WriteString(fmt.Sprintf("%q: %q, ", k, tagMap[k]))
					}
					b.WriteString("},\n")
				}
				b.WriteString(fmt.Sprintf("\t\t\tName: %q,\n", wb.name))
				b.WriteString("\t\t},\n")
				b.WriteString("\t\tsrv.MW,\n")
				b.WriteString(fmt.Sprintf("\t\tfunc() any { return new(%s) },\n", wb.inGo))
				b.WriteString("\t\tfunc(ctx *ssrpc.Context, in any) (any, error) {\n")
				b.WriteString(fmt.Sprintf("\t\t\treturn srv.Impl.%s(ctx, in.(*%s))\n", wb.method, wb.inGo))
				b.WriteString("\t\t},\n")
				b.WriteString("\t))\n\n")
			}
			b.WriteString("}\n\n")
		}

		// Phase-3 output: gRPC registration for methods with grpc=true.
		if len(grpcBinds) > 0 {
			b.WriteString(fmt.Sprintf("// Register%sToGRPC registers gRPC handlers for %s.\n", svc, svc))
			b.WriteString(fmt.Sprintf("func Register%sToGRPC(d *ssrpc.Dispatcher, srv %sSServer) {\n", svc, svc))
			b.WriteString("\tif d == nil || srv.Impl == nil {\n")
			b.WriteString("\t\treturn\n")
			b.WriteString("\t}\n\n")
			for _, gb := range grpcBinds {
				if gb.grpcServerStream {
					b.WriteString(fmt.Sprintf("\td.RegisterGRPCStream(%q, %q, ssrpc.WrapGRPCServerStreamTyped[*%s](\n", gb.grpcService, gb.method, gb.outGo))
				} else {
					b.WriteString(fmt.Sprintf("\td.RegisterGRPCUnary(%q, %q, func() any { return new(%s) }, ssrpc.WrapGRPCUnary(\n", gb.grpcService, gb.method, gb.inGo))
				}
				b.WriteString("\t\tssrpc.MethodDesc{\n")
				b.WriteString(fmt.Sprintf("\t\t\tCmd: %s,\n", gb.cmdLit))
				if gb.oneWay {
					b.WriteString("\t\t\tOneWay: true,\n")
				}
				if gb.uidLock {
					b.WriteString("\t\t\tUIDLock: true,\n")
				}
				if gb.auth {
					b.WriteString("\t\t\tAuth: true,\n")
				}
				if gb.sign {
					b.WriteString("\t\t\tSign: true,\n")
				}
				writeTimeoutField(&b, gb.timeoutMs)
				if tagMap := parseTraceTags(gb.tags); len(tagMap) > 0 {
					b.WriteString("\t\t\tTraceTags: map[string]string{")
					keys := make([]string, 0, len(tagMap))
					for k := range tagMap {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						b.WriteString(fmt.Sprintf("%q: %q, ", k, tagMap[k]))
					}
					b.WriteString("},\n")
				}
				b.WriteString(fmt.Sprintf("\t\t\tName: %q,\n", gb.name))
				b.WriteString("\t\t},\n")
				b.WriteString("\t\tsrv.MW,\n")
				if gb.grpcServerStream {
					b.WriteString(fmt.Sprintf("\t\tfunc() any { return new(%s) },\n", gb.inGo))
					b.WriteString(fmt.Sprintf("\t\tfunc(ctx *ssrpc.Context, in any, stream *ssrpc.ServerStream[*%s]) error {\n", gb.outGo))
					b.WriteString(fmt.Sprintf("\t\t\treturn srv.Impl.%s(ctx, in.(*%s), stream)\n", gb.method, gb.inGo))
					b.WriteString("\t\t},\n")
				} else {
					b.WriteString("\t\tfunc(ctx *ssrpc.Context, in any) (any, error) {\n")
					b.WriteString(fmt.Sprintf("\t\t\treturn srv.Impl.%s(ctx, in.(*%s))\n", gb.method, gb.inGo))
					b.WriteString("\t\t},\n")
				}
				b.WriteString("\t))\n\n")
			}
			b.WriteString("}\n\n")
		}

		// Unified dispatcher registration (cmd + http + ws + grpc).
		// This allows transports to mount from a single registry.
		if hasAnyCmdMethod || len(httpBinds) > 0 || hasAnyWSMethod || hasAnyGRPCMethod {
			b.WriteString(fmt.Sprintf("// Register%sToDispatcher registers cmd/http/ws/grpc bindings into a unified ssrpc.Dispatcher.\n", svc))
			b.WriteString(fmt.Sprintf("func Register%sToDispatcher(d *ssrpc.Dispatcher, srv %sSServer) {\n", svc, svc))
			b.WriteString("\tif d == nil || srv.Impl == nil {\n")
			b.WriteString("\t\treturn\n")
			b.WriteString("\t}\n\n")

			// cmd-bound methods
			for _, m := range annotated {
				ext, _, err := readSsRpcOption(m.GetOptions(), extType, extMsgDesc, extNum)
				if err != nil {
					return "", err
				}
				timeoutMs := effectiveTimeoutMs(ext.timeoutMs, serviceTimeoutMs)
				hasCmd := !(ext.cmd == 0 && ext.cmdEnum == 0 && strings.TrimSpace(ext.cmdName) == "")
				if !hasCmd {
					continue
				}
				cmdExpr := ""
				if ext.cmd != 0 {
					cmdExpr = fmt.Sprintf("g1_protocol.CMD(0x%X)", ext.cmd)
				} else if ext.cmdEnum != 0 {
					cmdExpr = fmt.Sprintf("g1_protocol.CMD(0x%X)", uint32(ext.cmdEnum))
				} else {
					cmdExpr = "g1_protocol." + strings.TrimSpace(ext.cmdName)
				}

				cmdRespExpr := ""
				if ext.cmdResp != 0 {
					cmdRespExpr = fmt.Sprintf("g1_protocol.CMD(0x%X)", ext.cmdResp)
				}

				method := m.GetName()
				inGo, _, err := typeRef(m.GetInputType())
				if err != nil {
					return "", err
				}

				b.WriteString(fmt.Sprintf("\td.RegisterCmd(%s, ssrpc.WrapUnary(\n", cmdExpr))
				b.WriteString("\t\tssrpc.MethodDesc{\n")
				b.WriteString(fmt.Sprintf("\t\t\tCmd: %s,\n", cmdExpr))
				if ext.cmdResp != 0 {
					b.WriteString(fmt.Sprintf("\t\t\tCmdResp: %s,\n", cmdRespExpr))
				}
				if ext.oneWay {
					b.WriteString("\t\t\tOneWay: true,\n")
				}
				if ext.uidLock {
					b.WriteString("\t\t\tUIDLock: true,\n")
				}
				if ext.auth {
					b.WriteString("\t\t\tAuth: true,\n")
				}
				if ext.sign {
					b.WriteString("\t\t\tSign: true,\n")
				}
				writeTimeoutField(&b, timeoutMs)
				if tagMap := parseTraceTags(ext.tags); len(tagMap) > 0 {
					b.WriteString("\t\t\tTraceTags: map[string]string{")
					keys := make([]string, 0, len(tagMap))
					for k := range tagMap {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						b.WriteString(fmt.Sprintf("%q: %q, ", k, tagMap[k]))
					}
					b.WriteString("},\n")
				}
				if ext.comment != "" {
					b.WriteString(fmt.Sprintf("\t\t\tName: %q,\n", ext.comment))
				} else {
					b.WriteString(fmt.Sprintf("\t\t\tName: %q,\n", svc+"."+method))
				}
				b.WriteString("\t\t},\n")
				b.WriteString("\t\tsrv.MW,\n")
				b.WriteString(fmt.Sprintf("\t\tfunc() any { return new(%s) },\n", inGo))
				b.WriteString("\t\tfunc(ctx *ssrpc.Context, in any) (any, error) {\n")
				b.WriteString(fmt.Sprintf("\t\t\treturn srv.Impl.%s(ctx, in.(*%s))\n", method, inGo))
				b.WriteString("\t\t},\n")
				b.WriteString("\t))\n\n")
			}

			// http-bound methods
			for _, hb := range httpBinds {
				b.WriteString(fmt.Sprintf("\td.RegisterHTTP(%q, %q, ssrpc.WrapHTTPGin(\n", hb.httpMethod, hb.path))
				b.WriteString("\t\tssrpc.MethodDesc{\n")
				b.WriteString(fmt.Sprintf("\t\t\tCmd: %s,\n", hb.cmdLit))
				if hb.oneWay {
					b.WriteString("\t\t\tOneWay: true,\n")
				}
				if hb.uidLock {
					b.WriteString("\t\t\tUIDLock: true,\n")
				}
				if hb.auth {
					b.WriteString("\t\t\tAuth: true,\n")
				}
				if hb.sign {
					b.WriteString("\t\t\tSign: true,\n")
				}
				writeTimeoutField(&b, hb.timeoutMs)
				if tagMap := parseTraceTags(hb.tags); len(tagMap) > 0 {
					b.WriteString("\t\t\tTraceTags: map[string]string{")
					keys := make([]string, 0, len(tagMap))
					for k := range tagMap {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						b.WriteString(fmt.Sprintf("%q: %q, ", k, tagMap[k]))
					}
					b.WriteString("},\n")
				}
				b.WriteString(fmt.Sprintf("\t\t\tName: %q,\n", hb.name))
				b.WriteString("\t\t},\n")
				b.WriteString("\t\tsrv.MW,\n")
				b.WriteString(fmt.Sprintf("\t\tfunc() any { return new(%s) },\n", hb.inGo))
				b.WriteString("\t\tfunc(ctx *ssrpc.Context, in any) (any, error) {\n")
				b.WriteString(fmt.Sprintf("\t\t\treturn srv.Impl.%s(ctx, in.(*%s))\n", hb.method, hb.inGo))
				b.WriteString("\t\t},\n")
				b.WriteString("\t))\n\n")
			}

			// ws-bound methods (CSPacket over WebSocket)
			for _, wb := range wsBinds {
				b.WriteString(fmt.Sprintf("\td.RegisterWS(uint32(%s), ssrpc.WrapWS(\n", wb.cmdLit))
				b.WriteString("\t\tssrpc.MethodDesc{\n")
				b.WriteString(fmt.Sprintf("\t\t\tCmd: %s,\n", wb.cmdLit))
				if wb.oneWay {
					b.WriteString("\t\t\tOneWay: true,\n")
				}
				if wb.uidLock {
					b.WriteString("\t\t\tUIDLock: true,\n")
				}
				if wb.auth {
					b.WriteString("\t\t\tAuth: true,\n")
				}
				if wb.sign {
					b.WriteString("\t\t\tSign: true,\n")
				}
				writeTimeoutField(&b, wb.timeoutMs)
				if tagMap := parseTraceTags(wb.tags); len(tagMap) > 0 {
					b.WriteString("\t\t\tTraceTags: map[string]string{")
					keys := make([]string, 0, len(tagMap))
					for k := range tagMap {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						b.WriteString(fmt.Sprintf("%q: %q, ", k, tagMap[k]))
					}
					b.WriteString("},\n")
				}
				b.WriteString(fmt.Sprintf("\t\t\tName: %q,\n", wb.name))
				b.WriteString("\t\t},\n")
				b.WriteString("\t\tsrv.MW,\n")
				b.WriteString(fmt.Sprintf("\t\tfunc() any { return new(%s) },\n", wb.inGo))
				b.WriteString("\t\tfunc(ctx *ssrpc.Context, in any) (any, error) {\n")
				b.WriteString(fmt.Sprintf("\t\t\treturn srv.Impl.%s(ctx, in.(*%s))\n", wb.method, wb.inGo))
				b.WriteString("\t\t},\n")
				b.WriteString("\t))\n\n")
			}

			// grpc-bound methods
			for _, gb := range grpcBinds {
				if gb.grpcServerStream {
					b.WriteString(fmt.Sprintf("\td.RegisterGRPCStream(%q, %q, ssrpc.WrapGRPCServerStreamTyped[*%s](\n", gb.grpcService, gb.method, gb.outGo))
				} else {
					b.WriteString(fmt.Sprintf("\td.RegisterGRPCUnary(%q, %q, func() any { return new(%s) }, ssrpc.WrapGRPCUnary(\n", gb.grpcService, gb.method, gb.inGo))
				}
				b.WriteString("\t\tssrpc.MethodDesc{\n")
				b.WriteString(fmt.Sprintf("\t\t\tCmd: %s,\n", gb.cmdLit))
				if gb.oneWay {
					b.WriteString("\t\t\tOneWay: true,\n")
				}
				if gb.uidLock {
					b.WriteString("\t\t\tUIDLock: true,\n")
				}
				if gb.auth {
					b.WriteString("\t\t\tAuth: true,\n")
				}
				if gb.sign {
					b.WriteString("\t\t\tSign: true,\n")
				}
				writeTimeoutField(&b, gb.timeoutMs)
				if tagMap := parseTraceTags(gb.tags); len(tagMap) > 0 {
					b.WriteString("\t\t\tTraceTags: map[string]string{")
					keys := make([]string, 0, len(tagMap))
					for k := range tagMap {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						b.WriteString(fmt.Sprintf("%q: %q, ", k, tagMap[k]))
					}
					b.WriteString("},\n")
				}
				b.WriteString(fmt.Sprintf("\t\t\tName: %q,\n", gb.name))
				b.WriteString("\t\t},\n")
				b.WriteString("\t\tsrv.MW,\n")
				if gb.grpcServerStream {
					b.WriteString(fmt.Sprintf("\t\tfunc() any { return new(%s) },\n", gb.inGo))
					b.WriteString(fmt.Sprintf("\t\tfunc(ctx *ssrpc.Context, in any, stream *ssrpc.ServerStream[*%s]) error {\n", gb.outGo))
					b.WriteString(fmt.Sprintf("\t\t\treturn srv.Impl.%s(ctx, in.(*%s), stream)\n", gb.method, gb.inGo))
					b.WriteString("\t\t},\n")
				} else {
					b.WriteString("\t\tfunc(ctx *ssrpc.Context, in any) (any, error) {\n")
					b.WriteString(fmt.Sprintf("\t\t\treturn srv.Impl.%s(ctx, in.(*%s))\n", gb.method, gb.inGo))
					b.WriteString("\t\t},\n")
				}
				b.WriteString("\t))\n\n")
			}
			b.WriteString("}\n\n")
		}

		// ---------------------------------------------------------------------
		// Registry binding. Generate <Service>Bindings(srv) []ssrpc.Binding
		// (the authoritative binding list) and Register<Service>ToRegistry, which
		// atomically registers the whole batch into a ssrpc.Registry. Production
		// code uses RegistryComponent + Register<Service>ToRegistry; the legacy
		// ToDispatcher/ToTransactionMgr are kept for compatibility.
		// ---------------------------------------------------------------------
		if hasAnyCmdMethod || len(httpBinds) > 0 || hasAnyWSMethod || hasAnyGRPCMethod {
			// <Service>Bindings returns the authoritative []ssrpc.Binding slice.
			b.WriteString(fmt.Sprintf("// %sBindings returns the authoritative ssrpc binding slice for %s.\n", svc, svc))
			b.WriteString(fmt.Sprintf("// Register%sToRegistry and the legacy Register%sToDispatcher both consume it.\n", svc, svc))
			b.WriteString(fmt.Sprintf("func %sBindings(srv %sSServer) []ssrpc.Binding {\n", svc, svc))
			b.WriteString("\tif srv.Impl == nil {\n\t\treturn nil\n\t}\n")
			b.WriteString("\treturn []ssrpc.Binding{\n")

			// cmd-bound methods -> BindingCMD
			for _, m := range annotated {
				ext, _, err := readSsRpcOption(m.GetOptions(), extType, extMsgDesc, extNum)
				if err != nil {
					return "", err
				}
				timeoutMs := effectiveTimeoutMs(ext.timeoutMs, serviceTimeoutMs)
				hasCmd := !(ext.cmd == 0 && ext.cmdEnum == 0 && strings.TrimSpace(ext.cmdName) == "")
				if !hasCmd {
					continue
				}
				cmdExpr := cmdLiteral(ext)
				method := m.GetName()
				inGo, _, err := typeRef(m.GetInputType())
				if err != nil {
					return "", err
				}
				b.WriteString("\t\t{Kind: ssrpc.BindingCMD, CMD: " + cmdExpr + ", CmdHandler: ssrpc.WrapUnary(\n")
				writeMethodDescForBinding(&b, ext, timeoutMs, svc+"."+method)
				b.WriteString("\t\t\tsrv.MW,\n")
				b.WriteString(fmt.Sprintf("\t\t\tfunc() any { return new(%s) },\n", inGo))
				b.WriteString("\t\t\tfunc(ctx *ssrpc.Context, in any) (any, error) {\n")
				b.WriteString(fmt.Sprintf("\t\t\t\treturn srv.Impl.%s(ctx, in.(*%s))\n", method, inGo))
				b.WriteString("\t\t\t},\n")
				b.WriteString("\t\t)},\n")
			}
			// http-bound methods -> BindingHTTP
			for _, hb := range httpBinds {
				b.WriteString(fmt.Sprintf("\t\t{Kind: ssrpc.BindingHTTP, HTTPMethod: %q, HTTPPath: %q, HTTPHandler: ssrpc.WrapHTTPGin(\n", hb.httpMethod, hb.path))
				writeMethodDescForBindingRaw(&b, hb.cmdLit, hb.oneWay, hb.uidLock, hb.auth, hb.sign, hb.timeoutMs, hb.tags, hb.name)
				b.WriteString("\t\t\tsrv.MW,\n")
				b.WriteString(fmt.Sprintf("\t\t\tfunc() any { return new(%s) },\n", hb.inGo))
				b.WriteString("\t\t\tfunc(ctx *ssrpc.Context, in any) (any, error) {\n")
				b.WriteString(fmt.Sprintf("\t\t\t\treturn srv.Impl.%s(ctx, in.(*%s))\n", hb.method, hb.inGo))
				b.WriteString("\t\t\t},\n")
				b.WriteString("\t\t)},\n")
			}
			// ws-bound methods -> BindingWS
			for _, wb := range wsBinds {
				b.WriteString("\t\t{Kind: ssrpc.BindingWS, CMD: " + wb.cmdLit + ", CmdHandler: ssrpc.WrapWS(\n")
				writeMethodDescForBindingRaw(&b, wb.cmdLit, wb.oneWay, wb.uidLock, wb.auth, wb.sign, wb.timeoutMs, wb.tags, wb.name)
				b.WriteString("\t\t\tsrv.MW,\n")
				b.WriteString(fmt.Sprintf("\t\t\tfunc() any { return new(%s) },\n", wb.inGo))
				b.WriteString("\t\t\tfunc(ctx *ssrpc.Context, in any) (any, error) {\n")
				b.WriteString(fmt.Sprintf("\t\t\t\treturn srv.Impl.%s(ctx, in.(*%s))\n", wb.method, wb.inGo))
				b.WriteString("\t\t\t},\n")
				b.WriteString("\t\t)},\n")
			}
			// grpc-bound methods -> BindingGRPCUnary / BindingGRPCStream
			for _, gb := range grpcBinds {
				if gb.grpcServerStream {
					b.WriteString(fmt.Sprintf("\t\t{Kind: ssrpc.BindingGRPCStream, GRPCService: %q, GRPCMethod: %q, StreamHandler: ssrpc.WrapGRPCServerStreamTyped[*%s](\n", gb.grpcService, gb.method, gb.outGo))
				} else {
					b.WriteString(fmt.Sprintf("\t\t{Kind: ssrpc.BindingGRPCUnary, GRPCService: %q, GRPCMethod: %q, ReqFactory: func() any { return new(%s) }, UnaryHandler: ssrpc.WrapGRPCUnary(\n", gb.grpcService, gb.method, gb.inGo))
				}
				writeMethodDescForBindingRaw(&b, gb.cmdLit, gb.oneWay, gb.uidLock, gb.auth, gb.sign, gb.timeoutMs, gb.tags, gb.name)
				b.WriteString("\t\t\tsrv.MW,\n")
				if gb.grpcServerStream {
					b.WriteString(fmt.Sprintf("\t\t\tfunc() any { return new(%s) },\n", gb.inGo))
					b.WriteString(fmt.Sprintf("\t\t\tfunc(ctx *ssrpc.Context, in any, stream *ssrpc.ServerStream[*%s]) error {\n", gb.outGo))
					b.WriteString(fmt.Sprintf("\t\t\t\treturn srv.Impl.%s(ctx, in.(*%s), stream)\n", gb.method, gb.inGo))
					b.WriteString("\t\t\t},\n")
				} else {
					b.WriteString("\t\t\tfunc(ctx *ssrpc.Context, in any) (any, error) {\n")
					b.WriteString(fmt.Sprintf("\t\t\t\treturn srv.Impl.%s(ctx, in.(*%s))\n", gb.method, gb.inGo))
					b.WriteString("\t\t\t},\n")
				}
				b.WriteString("\t\t)},\n")
			}
			b.WriteString("\t}\n")
			b.WriteString("}\n\n")

			// Register<Service>ToRegistry atomically registers the batch into a Registry.
			b.WriteString(fmt.Sprintf("// Register%sToRegistry atomically registers all %s bindings into r.\n", svc, svc))
			b.WriteString(fmt.Sprintf("// It is the production entry point: RegistryComponent calls it, then Seals r.\n"))
			b.WriteString(fmt.Sprintf("func Register%sToRegistry(r *ssrpc.Registry, srv %sSServer) error {\n", svc, svc))
			b.WriteString("\tif r == nil || srv.Impl == nil {\n")
			b.WriteString("\t\treturn ssrpc.ErrNilRegistry\n")
			b.WriteString("\t}\n")
			b.WriteString(fmt.Sprintf("\treturn r.Register(%q, %sBindings(srv)...)\n", svc, svc))
			b.WriteString("}\n\n")
		}

		// ---------------------------------------------------------------------
		// Client stub generation: type-safe RPC client for service-to-service calls.
		// Only generates for methods with cmd bindings.
		// ---------------------------------------------------------------------
		if hasAnyCmdMethod {
			b.WriteString(fmt.Sprintf("// %sClient provides type-safe RPC stubs for %s.\n", svc, svc))
			b.WriteString(fmt.Sprintf("// Methods derive the target server type from CMD automatically.\n"))
			b.WriteString(fmt.Sprintf("// ByRouter variants are also generated for callers that need explicit routerId routing.\n"))
			b.WriteString(fmt.Sprintf("// One-way methods additionally expose ByBusId/ByBusIdSimple and Simple helpers.\n"))
			b.WriteString(fmt.Sprintf("type %sClient struct{}\n\n", svc))
			b.WriteString(fmt.Sprintf("// New%sClient returns a new %sClient.\n", svc, svc))
			b.WriteString(fmt.Sprintf("func New%sClient() *%sClient {\n", svc, svc))
			b.WriteString(fmt.Sprintf("\treturn &%sClient{}\n", svc))
			b.WriteString("}\n\n")

			for _, m := range annotated {
				ext, _, err := readSsRpcOption(m.GetOptions(), extType, extMsgDesc, extNum)
				if err != nil {
					return "", err
				}
				hasCmd := !(ext.cmd == 0 && ext.cmdEnum == 0 && strings.TrimSpace(ext.cmdName) == "")
				if !hasCmd {
					continue
				}
				cmdExpr := ""
				if ext.cmd != 0 {
					cmdExpr = fmt.Sprintf("g1_protocol.CMD(0x%X)", ext.cmd)
				} else if ext.cmdEnum != 0 {
					cmdExpr = fmt.Sprintf("g1_protocol.CMD(0x%X)", uint32(ext.cmdEnum))
				} else {
					cmdExpr = "g1_protocol." + strings.TrimSpace(ext.cmdName)
				}

				method := m.GetName()
				inGo, _, err := typeRef(m.GetInputType())
				if err != nil {
					return "", err
				}
				outGo, _, err := typeRef(m.GetOutputType())
				if err != nil {
					return "", err
				}

				comment := ext.comment
				if comment == "" {
					comment = svc + "." + method
				}

				if ext.oneWay {
					// one_way: fire-and-forget, returns error only
					b.WriteString(fmt.Sprintf("// %s sends %s (one-way, no response).\n", method, comment))
					b.WriteString(fmt.Sprintf("func (c *%sClient) %s(ctx cmd_handler.IContext, req *%s) error {\n", svc, method, inGo))
					b.WriteString(fmt.Sprintf("\treturn ssrpc.SendByCmd(ctx, %s, req)\n", cmdExpr))
					b.WriteString("}\n\n")

					b.WriteString(fmt.Sprintf("// %sByRouter sends %s to an explicit routerId (one-way, no response).\n", method, comment))
					b.WriteString(fmt.Sprintf("func (c *%sClient) %sByRouter(ctx cmd_handler.IContext, routerId uint64, req *%s) error {\n", svc, method, inGo))
					b.WriteString(fmt.Sprintf("\treturn ssrpc.SendByCmdWithRouter(ctx, routerId, %s, req)\n", cmdExpr))
					b.WriteString("}\n\n")

					b.WriteString(fmt.Sprintf("// %sByBusId sends %s to an explicit busId (one-way, no response).\n", method, comment))
					b.WriteString(fmt.Sprintf("func (c *%sClient) %sByBusId(ctx cmd_handler.IContext, busId uint32, req *%s) error {\n", svc, method, inGo))
					b.WriteString(fmt.Sprintf("\treturn ssrpc.SendByCmdToBusId(ctx, busId, %s, req)\n", cmdExpr))
					b.WriteString("}\n\n")

					b.WriteString(fmt.Sprintf("// %sSimple sends %s without an IContext (one-way, no response).\n", method, comment))
					b.WriteString(fmt.Sprintf("func (c *%sClient) %sSimple(uid uint64, zone uint32, req *%s) error {\n", svc, method, inGo))
					b.WriteString(fmt.Sprintf("\treturn ssrpc.SendByCmdSimple(uid, zone, %s, req)\n", cmdExpr))
					b.WriteString("}\n\n")

					b.WriteString(fmt.Sprintf("// %sByBusIdSimple sends %s to an explicit busId without an IContext (one-way, no response).\n", method, comment))
					b.WriteString(fmt.Sprintf("func (c *%sClient) %sByBusIdSimple(busId uint32, uid uint64, req *%s) error {\n", svc, method, inGo))
					b.WriteString(fmt.Sprintf("\treturn ssrpc.SendByCmdToBusIdSimple(busId, uid, %s, req)\n", cmdExpr))
					b.WriteString("}\n\n")

					b.WriteString(fmt.Sprintf("// %sByRouterSimple sends %s to an explicit routerId without an IContext (one-way, no response).\n", method, comment))
					b.WriteString(fmt.Sprintf("func (c *%sClient) %sByRouterSimple(routerId, uid uint64, zone uint32, req *%s) error {\n", svc, method, inGo))
					b.WriteString(fmt.Sprintf("\treturn ssrpc.SendByCmdWithRouterSimple(routerId, uid, zone, %s, req)\n", cmdExpr))
					b.WriteString("}\n\n")
				} else {
					// request/response: synchronous call
					b.WriteString(fmt.Sprintf("// %s calls %s synchronously.\n", method, comment))
					b.WriteString(fmt.Sprintf("func (c *%sClient) %s(ctx cmd_handler.IContext, req *%s) (*%s, error) {\n", svc, method, inGo, outGo))
					b.WriteString(fmt.Sprintf("\trsp := &%s{}\n", outGo))
					b.WriteString(fmt.Sprintf("\tif err := ssrpc.CallByCmd(ctx, %s, req, rsp); err != nil {\n", cmdExpr))
					b.WriteString("\t\treturn nil, err\n")
					b.WriteString("\t}\n")
					b.WriteString("\treturn rsp, nil\n")
					b.WriteString("}\n\n")

					b.WriteString(fmt.Sprintf("// %sByRouter calls %s synchronously using an explicit routerId.\n", method, comment))
					b.WriteString(fmt.Sprintf("func (c *%sClient) %sByRouter(ctx cmd_handler.IContext, routerId uint64, req *%s) (*%s, error) {\n", svc, method, inGo, outGo))
					b.WriteString(fmt.Sprintf("\trsp := &%s{}\n", outGo))
					b.WriteString(fmt.Sprintf("\tif err := ssrpc.CallByCmdWithRouter(ctx, routerId, %s, req, rsp); err != nil {\n", cmdExpr))
					b.WriteString("\t\treturn nil, err\n")
					b.WriteString("\t}\n")
					b.WriteString("\treturn rsp, nil\n")
					b.WriteString("}\n\n")
				}
			}
		}
	}

	// Now prepend import block after collecting all imports.
	// NOTE: we build import block here and inject it after package declaration.
	// Because we already wrote content to builder, we do a simple rewrite.
	content := b.String()

	// construct final import list
	// NOTE: base imports are only needed if we generated any register wrapper.
	hasAnyWrapper := strings.Contains(content, "mgr.RegisterCmd(")
	hasGinWrapper := strings.Contains(content, "gin.IRoutes") && strings.Contains(content, "WrapHTTPGin(")
	hasDispatcher := strings.Contains(content, "Register") && strings.Contains(content, "ToDispatcher(d *ssrpc.Dispatcher")
	hasClientStub := strings.Contains(content, "cmd_handler.IContext")
	imports := []importSpec{
		{Path: "github.com/Iori372552686/GoOne/lib/service/ssrpc", Alias: ""},
	}
	basePaths := map[string]struct{}{
		"github.com/Iori372552686/GoOne/lib/service/ssrpc": {},
	}
	if hasAnyWrapper {
		imports = append(imports,
			importSpec{Path: "github.com/Iori372552686/GoOne/lib/service/transaction", Alias: ""},
			importSpec{Path: gameProtocolPath, Alias: "g1_protocol"},
		)
		basePaths["github.com/Iori372552686/GoOne/lib/service/transaction"] = struct{}{}
		basePaths[gameProtocolPath] = struct{}{}
	}
	if hasGinWrapper {
		imports = append(imports, importSpec{Path: "github.com/gin-gonic/gin", Alias: "gin"})
		basePaths["github.com/gin-gonic/gin"] = struct{}{}
	}
	if hasClientStub {
		cmdHandlerPath := "github.com/Iori372552686/GoOne/lib/api/cmd_handler"
		if _, dup := basePaths[cmdHandlerPath]; !dup {
			imports = append(imports, importSpec{Path: cmdHandlerPath, Alias: "cmd_handler"})
			basePaths[cmdHandlerPath] = struct{}{}
		}
	}
	if strings.Contains(content, "time.Millisecond") {
		if _, dup := basePaths["time"]; !dup {
			imports = append(imports, importSpec{Path: "time", Alias: ""})
			basePaths["time"] = struct{}{}
		}
	}

	// If dispatcher registration is present, ensure ssrpc is imported (already) and
	// only add g1_protocol when cmd expressions are referenced.
	_ = hasDispatcher
	// deterministic import order
	extra := make([]string, 0, len(usedMsgImports))
	for impPath := range usedMsgImports {
		if _, dup := basePaths[impPath]; dup {
			continue
		}
		extra = append(extra, impPath)
	}
	sort.Strings(extra)
	for _, impPath := range extra {
		alias := ib.byPath[impPath]
		if alias == "" {
			alias = ib.add(impPath, filepath.Base(impPath))
		}
		imports = append(imports, importSpec{Path: impPath, Alias: alias})
	}

	var iblk strings.Builder
	iblk.WriteString("import (\n")
	for _, im := range imports {
		if im.Alias != "" {
			iblk.WriteString(fmt.Sprintf("\t%s \"%s\"\n", im.Alias, im.Path))
		} else {
			iblk.WriteString(fmt.Sprintf("\t\"%s\"\n", im.Path))
		}
	}
	iblk.WriteString(")\n\n")

	// inject import block after "package xxx\n\n"
	pkgDecl := "package " + goPkgName + "\n\n"
	if !strings.Contains(content, pkgDecl) {
		return "", fmt.Errorf("internal: cannot find package decl to inject imports")
	}
	content = strings.Replace(content, pkgDecl, pkgDecl+iblk.String(), 1)

	return content, nil
}

type ssrpcOpt struct {
	cmd         uint32
	cmdResp     uint32
	oneWay      bool
	uidLock     bool
	cmdEnum     int32
	cmdName     string
	auth        bool
	sign        bool
	timeoutMs   uint32
	tags        []string
	httpPath    string
	httpMethod  string
	ws          bool
	grpc        bool
	grpcService string
	comment     string
}

type ssrpcServiceOpt struct {
	timeoutMs uint32
}

func buildSsRpcExtension(fds []*descriptorpb.FileDescriptorProto) (protoreflect.ExtensionType, protoreflect.MessageDescriptor, protowire.Number, error) {
	return buildMessageExtension(fds, ssrpcExtFullName, ssrpcOptFilePath, "ssrpc")
}

func buildSsRpcServiceExtension(fds []*descriptorpb.FileDescriptorProto) (protoreflect.ExtensionType, protoreflect.MessageDescriptor, protowire.Number, error) {
	extType, msgDesc, num, err := buildMessageExtension(fds, ssrpcServiceExtFullName, ssrpcOptFilePath, "ssrpc_service")
	if err != nil && strings.Contains(err.Error(), "cannot find ssrpc extension") {
		return nil, nil, 0, nil
	}
	return extType, msgDesc, num, err
}

func buildMessageExtension(fds []*descriptorpb.FileDescriptorProto, fullName, filePath, shortName string) (protoreflect.ExtensionType, protoreflect.MessageDescriptor, protowire.Number, error) {
	set := &descriptorpb.FileDescriptorSet{File: fds}
	files, err := protodesc.NewFiles(set)
	if err != nil {
		return nil, nil, 0, err
	}

	// locate extension descriptor by full name (best-effort).
	var extDesc protoreflect.ExtensionDescriptor
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		exts := fd.Extensions()
		for i := 0; i < exts.Len(); i++ {
			e := exts.Get(i)
			if string(e.FullName()) == fullName {
				extDesc = e
				return false
			}
		}
		return true
	})

	if extDesc == nil {
		// fallback: try to locate by known file path + name.
		if fd, err2 := files.FindFileByPath(filePath); err2 == nil {
			extDesc = fd.Extensions().ByName(protoreflect.Name(shortName))
		}
	}
	if extDesc == nil {
		return nil, nil, 0, fmt.Errorf("cannot find ssrpc extension %q (did you import %q?)", fullName, filePath)
	}

	extType := dynamicpb.NewExtensionType(extDesc)
	msgDesc := extDesc.Message()
	if msgDesc == nil {
		return nil, nil, 0, errors.New("ssrpc extension is not a message type")
	}
	return extType, msgDesc, protowire.Number(extDesc.Number()), nil
}

func readSsRpcOption(opts *descriptorpb.MethodOptions, extType protoreflect.ExtensionType, extMsgDesc protoreflect.MessageDescriptor, extNum protowire.Number) (ssrpcOpt, bool, error) {
	m, ok, err := readExtensionMessage(opts, extType, extMsgDesc, extNum)
	if err != nil || !ok {
		return ssrpcOpt{}, ok, err
	}
	return parseSsRpcOpt(m, extMsgDesc), true, nil
}

func readSsRpcServiceOption(opts *descriptorpb.ServiceOptions, extType protoreflect.ExtensionType, extMsgDesc protoreflect.MessageDescriptor, extNum protowire.Number) (ssrpcServiceOpt, bool, error) {
	m, ok, err := readExtensionMessage(opts, extType, extMsgDesc, extNum)
	if err != nil || !ok {
		return ssrpcServiceOpt{}, ok, err
	}
	return ssrpcServiceOpt{timeoutMs: getUint32(m, extMsgDesc.Fields().ByName("timeout_ms"))}, true, nil
}

func readExtensionMessage(opts proto.Message, extType protoreflect.ExtensionType, extMsgDesc protoreflect.MessageDescriptor, extNum protowire.Number) (protoreflect.Message, bool, error) {
	if opts == nil || extType == nil || extMsgDesc == nil || extNum == 0 {
		return nil, false, nil
	}
	// Two encoding modes to support:
	// 1) In-memory extension map (e.g. tests using proto.SetExtension).
	// 2) Unknown fields (what protoc typically sends in CodeGeneratorRequest).
	//
	// For dynamic message-typed extensions, proto.GetExtension may panic when the
	// extension is ABSENT (it tries to materialize an invalid zero-value message).
	// So we must only call GetExtension when we KNOW it exists.

	// Fast path: already decoded into extension map.
	if proto.HasExtension(opts, extType) {
		v := proto.GetExtension(opts, extType)
		pm, ok := v.(proto.Message)
		if !ok || pm == nil {
			return nil, true, fmt.Errorf("ssrpc extension value type mismatch: %T", v)
		}
		return pm.ProtoReflect(), true, nil
	}

	// Slow path: scan unknown fields and decode payload bytes.
	raw, ok, err := findLenDelimitedField(opts.ProtoReflect().GetUnknown(), extNum)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	dm := dynamicpb.NewMessage(extMsgDesc)
	if err := proto.Unmarshal(raw, dm); err != nil {
		return nil, true, fmt.Errorf("failed to unmarshal ssrpc option: %w", err)
	}

	return dm.ProtoReflect(), true, nil
}

func parseSsRpcOpt(m protoreflect.Message, extMsgDesc protoreflect.MessageDescriptor) ssrpcOpt {
	// cmd
	cmd := getUint32(m, extMsgDesc.Fields().ByName("cmd"))
	cmdResp := getUint32(m, extMsgDesc.Fields().ByName("cmd_resp"))
	oneWay := getBool(m, extMsgDesc.Fields().ByName("one_way"))
	uidLock := getBool(m, extMsgDesc.Fields().ByName("uid_lock"))
	cmdEnum := getInt32(m, extMsgDesc.Fields().ByName("cmd_enum"))
	cmdName := getString(m, extMsgDesc.Fields().ByName("cmd_name"))
	auth := getBool(m, extMsgDesc.Fields().ByName("auth"))
	sign := getBool(m, extMsgDesc.Fields().ByName("sign"))
	timeoutMs := getUint32(m, extMsgDesc.Fields().ByName("timeout_ms"))
	tags := getStringList(m, extMsgDesc.Fields().ByName("trace_tags"))
	httpPath := getString(m, extMsgDesc.Fields().ByName("http_path"))
	httpMethod := getString(m, extMsgDesc.Fields().ByName("http_method"))
	ws := getBool(m, extMsgDesc.Fields().ByName("ws"))
	grpcFlag := getBool(m, extMsgDesc.Fields().ByName("grpc"))
	grpcService := getString(m, extMsgDesc.Fields().ByName("grpc_service"))
	comment := getString(m, extMsgDesc.Fields().ByName("comment"))

	return ssrpcOpt{
		cmd: cmd, cmdResp: cmdResp, oneWay: oneWay, uidLock: uidLock,
		cmdEnum: cmdEnum, cmdName: cmdName,
		auth: auth, sign: sign, timeoutMs: timeoutMs, tags: tags,
		httpPath: httpPath, httpMethod: httpMethod,
		ws: ws, grpc: grpcFlag, grpcService: grpcService,
		comment: comment,
	}
}

func writeTimeoutField(b *strings.Builder, timeoutMs uint32) {
	if timeoutMs == 0 {
		return
	}
	b.WriteString(fmt.Sprintf("\t\t\tTimeout: %d * time.Millisecond,\n", timeoutMs))
}

// cmdLiteral 返回 ext 的 cmd 表达式字面量（如 g1_protocol.CMD(0x12) 或
// g1_protocol.CMD_MAIN_LOGIN_REQ）。用于 binding 生成。
func cmdLiteral(ext ssrpcOpt) string {
	if ext.cmd != 0 {
		return fmt.Sprintf("g1_protocol.CMD(0x%X)", ext.cmd)
	}
	if ext.cmdEnum != 0 {
		return fmt.Sprintf("g1_protocol.CMD(0x%X)", uint32(ext.cmdEnum))
	}
	return "g1_protocol." + strings.TrimSpace(ext.cmdName)
}

// writeMethodDescForBinding 写入一个 ssrpc.MethodDesc{...} 字面量（用于 binding 生成），
// 从 ssrpcOpt 取字段。
func writeMethodDescForBinding(b *strings.Builder, ext ssrpcOpt, timeoutMs uint32, name string) {
	b.WriteString("\t\t\tssrpc.MethodDesc{\n")
	b.WriteString(fmt.Sprintf("\t\t\t\tCmd: %s,\n", cmdLiteral(ext)))
	if ext.cmdResp != 0 {
		b.WriteString(fmt.Sprintf("\t\t\t\tCmdResp: g1_protocol.CMD(0x%X),\n", ext.cmdResp))
	}
	if ext.oneWay {
		b.WriteString("\t\t\t\tOneWay: true,\n")
	}
	if ext.uidLock {
		b.WriteString("\t\t\t\tUIDLock: true,\n")
	}
	if ext.auth {
		b.WriteString("\t\t\t\tAuth: true,\n")
	}
	if ext.sign {
		b.WriteString("\t\t\t\tSign: true,\n")
	}
	if timeoutMs != 0 {
		b.WriteString(fmt.Sprintf("\t\t\t\tTimeout: %d * time.Millisecond,\n", timeoutMs))
	}
	if tagMap := parseTraceTags(ext.tags); len(tagMap) > 0 {
		b.WriteString("\t\t\t\tTraceTags: map[string]string{")
		keys := make([]string, 0, len(tagMap))
		for k := range tagMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("%q: %q, ", k, tagMap[k]))
		}
		b.WriteString("},\n")
	}
	b.WriteString(fmt.Sprintf("\t\t\t\tName: %q,\n", name))
	b.WriteString("\t\t\t},\n")
}

// writeMethodDescForBindingRaw 与 writeMethodDescForBinding 相同，但从已解构的原始字段
// 构造（用于 http/ws/grpc bind，它们不持有 ssrpcOpt）。
func writeMethodDescForBindingRaw(b *strings.Builder, cmdLit string, oneWay, uidLock, auth, sign bool, timeoutMs uint32, tags []string, name string) {
	b.WriteString("\t\t\tssrpc.MethodDesc{\n")
	b.WriteString(fmt.Sprintf("\t\t\t\tCmd: %s,\n", cmdLit))
	if oneWay {
		b.WriteString("\t\t\t\tOneWay: true,\n")
	}
	if uidLock {
		b.WriteString("\t\t\t\tUIDLock: true,\n")
	}
	if auth {
		b.WriteString("\t\t\t\tAuth: true,\n")
	}
	if sign {
		b.WriteString("\t\t\t\tSign: true,\n")
	}
	if timeoutMs != 0 {
		b.WriteString(fmt.Sprintf("\t\t\t\tTimeout: %d * time.Millisecond,\n", timeoutMs))
	}
	if tagMap := parseTraceTags(tags); len(tagMap) > 0 {
		b.WriteString("\t\t\t\tTraceTags: map[string]string{")
		keys := make([]string, 0, len(tagMap))
		for k := range tagMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("%q: %q, ", k, tagMap[k]))
		}
		b.WriteString("},\n")
	}
	b.WriteString(fmt.Sprintf("\t\t\t\tName: %q,\n", name))
	b.WriteString("\t\t\t},\n")
}

func effectiveTimeoutMs(methodTimeoutMs, serviceTimeoutMs uint32) uint32 {
	if methodTimeoutMs != 0 {
		return methodTimeoutMs
	}
	if serviceTimeoutMs != 0 {
		return serviceTimeoutMs
	}
	return defaultSSRPCTimeoutMs
}

// findLenDelimitedField finds the first length-delimited (bytes) field for the given field number
// inside an unknown-field buffer.
func findLenDelimitedField(b []byte, num protowire.Number) ([]byte, bool, error) {
	for len(b) > 0 {
		n, wt, tagLen := protowire.ConsumeTag(b)
		if tagLen < 0 {
			return nil, false, fmt.Errorf("invalid unknown tag: %v", protowire.ParseError(tagLen))
		}
		b = b[tagLen:]
		switch wt {
		case protowire.VarintType:
			_, vLen := protowire.ConsumeVarint(b)
			if vLen < 0 {
				return nil, false, fmt.Errorf("invalid unknown varint: %v", protowire.ParseError(vLen))
			}
			b = b[vLen:]
		case protowire.Fixed32Type:
			_, vLen := protowire.ConsumeFixed32(b)
			if vLen < 0 {
				return nil, false, fmt.Errorf("invalid unknown fixed32: %v", protowire.ParseError(vLen))
			}
			b = b[vLen:]
		case protowire.Fixed64Type:
			_, vLen := protowire.ConsumeFixed64(b)
			if vLen < 0 {
				return nil, false, fmt.Errorf("invalid unknown fixed64: %v", protowire.ParseError(vLen))
			}
			b = b[vLen:]
		case protowire.BytesType:
			v, vLen := protowire.ConsumeBytes(b)
			if vLen < 0 {
				return nil, false, fmt.Errorf("invalid unknown bytes: %v", protowire.ParseError(vLen))
			}
			b = b[vLen:]
			if n == num {
				return v, true, nil
			}
		case protowire.StartGroupType:
			_, vLen := protowire.ConsumeGroup(n, b)
			if vLen < 0 {
				return nil, false, fmt.Errorf("invalid unknown group: %v", protowire.ParseError(vLen))
			}
			b = b[vLen:]
		default:
			return nil, false, fmt.Errorf("unknown wire type %v", wt)
		}
	}
	return nil, false, nil
}

func getUint32(m protoreflect.Message, fd protoreflect.FieldDescriptor) uint32 {
	if fd == nil {
		return 0
	}
	if !m.Has(fd) {
		return 0
	}
	return uint32(m.Get(fd).Uint())
}

func getInt32(m protoreflect.Message, fd protoreflect.FieldDescriptor) int32 {
	if fd == nil {
		return 0
	}
	if !m.Has(fd) {
		return 0
	}
	v := m.Get(fd)
	switch fd.Kind() {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return int32(v.Int())
	case protoreflect.EnumKind:
		return int32(v.Enum())
	default:
		return 0
	}
}

func getBool(m protoreflect.Message, fd protoreflect.FieldDescriptor) bool {
	if fd == nil {
		return false
	}
	if !m.Has(fd) {
		return false
	}
	return m.Get(fd).Bool()
}

func getString(m protoreflect.Message, fd protoreflect.FieldDescriptor) string {
	if fd == nil {
		return ""
	}
	if !m.Has(fd) {
		return ""
	}
	return m.Get(fd).String()
}

func getStringList(m protoreflect.Message, fd protoreflect.FieldDescriptor) []string {
	if fd == nil {
		return nil
	}
	if fd.Cardinality() != protoreflect.Repeated || fd.Kind() != protoreflect.StringKind {
		return nil
	}
	if !m.Has(fd) {
		return nil
	}
	l := m.Get(fd).List()
	if l.Len() == 0 {
		return nil
	}
	out := make([]string, 0, l.Len())
	for i := 0; i < l.Len(); i++ {
		out = append(out, l.Get(i).String())
	}
	return out
}

// parseTraceTags parses "k=v" pairs, keeping the last value for duplicate keys.
func parseTraceTags(tags []string) map[string]string {
	out := map[string]string{}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		parts := strings.SplitN(t, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		if k == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func goTypeNameFromProtoType(filePkg string, fullType string) (string, error) {
	// fullType: ".pkg.Msg" or ".pkg.Outer.Inner"
	fullType = strings.TrimSpace(fullType)
	fullType = strings.TrimPrefix(fullType, ".")
	if fullType == "" {
		return "", fmt.Errorf("empty proto type")
	}
	parts := strings.Split(fullType, ".")
	filePkgParts := strings.Split(filePkg, ".")
	if len(parts) <= len(filePkgParts) {
		return "", fmt.Errorf("invalid proto type %q for file package %q", fullType, filePkg)
	}
	// Phase A: expect input/output messages in the same proto package.
	for i := 0; i < len(filePkgParts); i++ {
		if parts[i] != filePkgParts[i] {
			return "", fmt.Errorf("cross-package message not supported yet (filePkg=%q type=%q)", filePkg, fullType)
		}
	}
	msgParts := parts[len(filePkgParts):]
	// Protoc-gen-go maps nested messages to Outer_Inner.
	return strings.Join(msgParts, "_"), nil
}

func guessGoPkgName(goPackageOpt string, protoPkg string) string {
	// go_package format: "path/to/pkg;name"
	if goPackageOpt != "" {
		if i := strings.LastIndex(goPackageOpt, ";"); i >= 0 && i+1 < len(goPackageOpt) {
			return sanitizeGoIdent(goPackageOpt[i+1:])
		}
		base := filepath.Base(goPackageOpt)
		if base != "" && base != "." && base != "/" {
			return sanitizeGoIdent(base)
		}
	}

	// proto package: a.b.c -> c
	if protoPkg != "" {
		parts := strings.Split(protoPkg, ".")
		return sanitizeGoIdent(parts[len(parts)-1])
	}
	return "gen"
}

func sanitizeGoIdent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "gen"
	}
	// minimal sanitize: replace '-' with '_'
	s = strings.ReplaceAll(s, "-", "_")
	return s
}
