package utilities

//
//func (rh *rowheap.RowHeap) Output(output *csv.Writer, hasHeader bool, header []string) {
//	if hasHeader {
//		log.Printf("%s\n", header)
//		err := output.Write(header)
//		if err != nil {
//			panic("error while writing to file")
//		}
//	}
//	for rh.Len() > 0 {
//		row := heap.Pop(rh).(row.Row)
//		log.Printf("%s %d\n", row.Columns, rh.Len())
//		err := output.Write(row.Columns)
//		if err != nil {
//			panic("error while writing to file")
//		}
//	}
//	output.Flush()
//}
