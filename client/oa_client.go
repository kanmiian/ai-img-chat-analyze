package client

import (
	"net/http"
	"time"
)

// OaClient 负责与 OA 系统的 API 通信
type OaClient struct {
	baseURL    string
	httpClient *http.Client
}

// EmployeeData (OA系统返回的员工基准数据)
type EmployeeData struct {
	UserId string `json:"user_id"`
	Alias  string `json:"alias"`
}

func NewOaClient(baseURL string) *OaClient {
	return &OaClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetEmployeeData 从 OA 系统获取员工的基准数据
func (c *OaClient) GetEmployeeData(employeeID string) (*EmployeeData, error) {
	//
	// ---- 生产环境代码 (示例) ----
	// reqURL := fmt.Sprintf("%s/employees/%s", c.baseURL, employeeID)
	// req, err := http.NewRequest("GET", reqURL, nil)
	// if err != nil {
	// 	return nil, err
	// }
	// // (!! 可能需要添加认证，例如 Bearer Token)
	// // req.Header.Set("Authorization", "Bearer YOUR_OA_API_TOKEN")
	//
	// resp, err := c.httpClient.Do(req)
	// if err != nil {
	// 	return nil, err
	// }
	// defer resp.Body.Close()
	//
	// if resp.StatusCode != http.StatusOK {
	// 	return nil, fmt.Errorf("OA API 返回错误: %d", resp.StatusCode)
	// }
	//
	// var data EmployeeData
	// if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
	// 	return nil, err
	// }
	// return &data, nil
	//

	// 临时返回模拟数据（开发环境）
	return &EmployeeData{
		UserId: employeeID,
		Alias:  "测试员工",
	}, nil
}
