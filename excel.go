package wmfw

import "github.com/xyzj/gopsu/excel"

// // AddRow 添加行
// // cells： 每个单元格的数据，任意格式
// func (e *gopsu.ExcelData) AddRow(cells ...interface{}) {
// 	row := e.xlsxSheet.AddRow()
// 	row.SetHeight(15)
// 	// row.WriteSlice(cells, -1)
// 	for _, v := range cells {
// 		row.AddCell().SetValue(v)
// 	}
// }

// // SetColume 设置列头
// // columeName: 列头名，有多少写多少个
// func (e *gopsu.ExcelData) SetColume(columeName ...string) {
// 	row := e.xlsxSheet.AddRow()
// 	row.SetHeight(20)
// 	for _, v := range columeName {
// 		cell := row.AddCell()
// 		cell.SetStyle(e.colStyle)
// 		cell.SetString(v)
// 	}
// }

// // Write 将excel数据写入到writer
// // w： io.writer
// func (e *gopsu.ExcelData) Write(w io.Writer) error {
// 	return e.xlsxFile.Write(w)
// }

// // Save 保存excel数据到文件
// // 返回保存的完整文件名，错误
// func (e *gopsu.ExcelData) Save() (string, error) {
// 	// 判断文件夹是否存在
// 	if !pathtool.IsExist(filepath.Join(gopsu.DefaultCacheDir, "excel")) {
// 		err := os.Mkdir(filepath.Join(gopsu.DefaultCacheDir, "excel"), 0755)
// 		if err != nil {
// 			return "", fmt.Errorf("excel-导出文件夹创建失败:" + err.Error())
// 		}
// 	}
// 	err := e.xlsxFile.Save(filepath.Join(gopsu.DefaultCacheDir, "excel", e.fileName))
// 	if err != nil {
// 		return "", fmt.Errorf("excel-文件保存失败:" + err.Error())
// 	}
// 	return e.fileName, nil
// }

// NewExcel 创建新的excel文件，Deprecated, use excel.NewExcel
func (fw *WMFrameWorkV2) NewExcel(filename string) (*excel.FileData, error) {
	return excel.NewExcel(filename)
	// var err error
	// e := &gopsu.ExcelData{
	// 	xlsxFile: xlsx.NewFile(),
	// 	colStyle: xlsx.NewStyle(),
	// }
	// e.colStyle.Alignment.Horizontal = "center"
	// e.colStyle.Font.Bold = true
	// e.colStyle.ApplyAlignment = true
	// e.colStyle.ApplyFont = true
	// e.xlsxSheet, err = e.xlsxFile.AddSheet(filename + time.Now().Format("2006-01-02"))
	// if err != nil {
	// 	return nil, fmt.Errorf("excel-sheet创建失败:" + err.Error())
	// }
	// e.fileName = filename + time.Now().Format("2006-01-02-15-04-05") + ".xlsx"
	// return e, nil
}

// type xlsxRow struct {
// 	Row  *xlsx.Row
// 	Data []string
// }

// func newRow(row *xlsx.Row, data []string) *xlsxRow {
// 	row.SetHeight(15)
// 	return &xlsxRow{
// 		Row:  row,
// 		Data: data,
// 	}
// }

// func (row *xlsxRow) setRowTitle() error {
// 	return generateRow(row.Row, row.Data)
// }

// func (row *xlsxRow) generateRow() error {
// 	return generateRow(row.Row, row.Data)
// }

// func generateRow(row *xlsx.Row, rowStr []string) error {
// 	if rowStr == nil {
// 		return fmt.Errorf("no data to generate xlsx")
// 	}
// 	for _, v := range rowStr {
// 		cell := row.AddCell()
// 		cell.SetString(v)
// 	}
// 	return nil
// }

// // Export2Xlsx 数据导出到xlsx文件
// //  fileName: 导出的文件名，不需要添加扩展名
// //  columename: 列标题
// //  cells: 单元格数据，二维字符串数组
// func (fw *WMFrameWorkV2) Export2Xlsx(fileName string, columeName []string, cells [][]string) (string, error) {
// 	file := xlsx.NewFile()
// 	sheet, err := file.AddSheet(fileName + time.Now().Format("2006-01-02"))
// 	if err != nil {
// 		return "", fmt.Errorf("excel-sheet创建失败:" + err.Error())
// 	}

// 	fileName = fileName + time.Now().Format("2006-01-02-15-04-05")

// 	titleRow := sheet.AddRow()
// 	xlsRow := newRow(titleRow, columeName)
// 	err = xlsRow.setRowTitle()
// 	if err != nil {
// 		return "", fmt.Errorf("excel-表头创建失败:" + err.Error())
// 	}

// 	for _, cell := range cells {
// 		currentRow := sheet.AddRow()
// 		tmp := make([]string, 0)
// 		tmp = append(tmp, cell...)
// 		xlsRow := newRow(currentRow, tmp)
// 		err := xlsRow.generateRow()
// 		if err != nil {
// 			return "", fmt.Errorf("excel-内容填充失败:" + err.Error())
// 		}
// 	}
// 	// 判断文件夹是否存在
// 	if !pathtool.IsExist(filepath.Join(gopsu.DefaultCacheDir, "excel")) {
// 		err := os.Mkdir(filepath.Join(gopsu.DefaultCacheDir, "excel"), 0755)
// 		if err != nil {
// 			return "", fmt.Errorf("excel-导出文件夹创建失败:" + err.Error())
// 		}
// 	}

// 	err = file.Save(filepath.Join(gopsu.DefaultCacheDir, "excel", fileName+".xlsx"))
// 	if err != nil {
// 		return "", fmt.Errorf("excel-文件保存失败:" + err.Error())
// 	} else {
// 		return fileName + ".xlsx", nil
// 	}
// }
