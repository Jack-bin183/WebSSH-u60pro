package service

import (
	"gossh/app/utils"
	// "log/slog"
	"gossh/gin"
	"net/http"
	"sort"
	"sync"
	"time"
)

type ZteRPCRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	Id      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type ZteRPCResponse struct {
	Jsonrpc string        `json:"jsonrpc"`
	Id      int           `json:"id"`
	Result  []interface{} `json:"result"`
	CostMs  int64         `json:"cost_ms"` // ✅ ubus 执行耗时
}

func ZteUbusBatchHandler(c *gin.Context) {
	var req []ZteRPCRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "invalid json-rpc request",
		})
		return
	}

	result := CallUbusBatchAsync(req)

	// ⚠ 官方就是直接返回数组
	c.JSON(http.StatusOK, result)
}

func CallUbusBatchAsync( reqs []ZteRPCRequest ) [] ZteRPCResponse {
	const maxConcurrent = 12 // ✅ 推荐用 const [5-10]个并发
	var (
		wg  sync.WaitGroup
		sem = make(chan struct{}, maxConcurrent)
		out = make(chan ZteRPCResponse, len(reqs))
	)

	for _, rpc := range reqs {
		wg.Add(1)

		go func(r ZteRPCRequest) {
    	defer wg.Done()

    	sem <- struct{}{}
    	defer func() { <-sem }()

    	if len(r.Params) < 4 {
    		out <- ZteRPCResponse{
    			Jsonrpc: "2.0",
    			Id:      r.Id,
    			CostMs:  0,
    			Result:  []interface{}{1, "invalid params"},
    		}
    		return
    	}

    	service, _ := r.Params[1].(string)
    	method, _ := r.Params[2].(string)
    	params, _ := r.Params[3].(map[string]interface{})

    	start := time.Now()

    	data, err := utils.GetDataFromUbus(service, method, params)

    	costMs := time.Since(start).Milliseconds()

    	if err != nil {
    		out <- ZteRPCResponse{
    			Jsonrpc: "2.0",
    			Id:      r.Id,
    			CostMs:  costMs,
    			Result:  []interface{}{1, err.Error()},
    		}
    		return
    	}

    	out <- ZteRPCResponse{
    		Jsonrpc: "2.0",
    		Id:      r.Id,
    		CostMs:  costMs,
    		Result:  []interface{}{0, data},
    	}
    }(rpc)
	}

	// 等待完成
	wg.Wait()
	close(out)

	// 收集结果
	results := make([]ZteRPCResponse, 0, len(reqs))
	for r := range out {
		results = append(results, r)
	}

	// 🔥 按 id 排序（官方风格）
	sort.Slice(results, func(i, j int) bool {
		return results[i].Id < results[j].Id
	})

	return results
}


// UbusAction 用于调用 ubus 并返回结果
// func UbusAction(c *gin.Context) {
// 	// 请求参数结构体
// 	type ubusRequest struct {
// 		Service string                 `form:"service" json:"service" binding:"required"`
// 		Method  string                 `form:"method"  json:"method"  binding:"required"`
// 		Params  map[string]interface{} `form:"params"  json:"params"`
// 	}

// 	var req ubusRequest
// 	if err := c.ShouldBind(&req); err != nil {
// 		slog.Error("绑定数据错误", "err_msg", err.Error())
// 		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "输入数据不合法"})
// 		return
// 	}

// 	// 调用 utils.getDataFromUbus
// 	result, err := utils.GetDataFromUbus(req.Service, req.Method, req.Params)
// 	if err != nil {
// 		slog.Error("Ubus 调用失败", "err_msg", err.Error())
// 		c.JSON(http.StatusOK, gin.H{"code": 2, "msg": "ubus 调用失败", "err": err.Error()})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"code":   0,
// 		"msg":    "success",
// 		"result": result,
// 	})
// }
